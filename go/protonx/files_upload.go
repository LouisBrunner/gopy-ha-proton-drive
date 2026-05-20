package protonx

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/go-resty/resty/v2"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

func (me *Extension) UploadFileByReader(ctx context.Context, parentLinkID string, filename string, modTime time.Time, file io.Reader) error {
	parentLink, err := me.GetLink(ctx, parentLinkID)
	if err != nil {
		return err
	}

	return me.uploadFile(ctx, parentLink, filename, modTime, file)
}

func (me *Extension) UploadFileByPath(ctx context.Context, parentLink *proton.Link, filename string, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}

	in := bufio.NewReader(f)

	return me.uploadFile(ctx, parentLink, filename, info.ModTime(), in)
}

func (me *Extension) uploadFile(ctx context.Context, parentLink *proton.Link, filename string, modTime time.Time, file io.Reader) error {
	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		mimeType = "text/plain"
	}

	draft, err := me.createUploadDraft(ctx, parentLink, filename, modTime, mimeType)
	if err != nil {
		return err
	}

	manifest, xattr, err := me.uploadBlocks(ctx, draft, file)
	if err != nil {
		return err
	}

	xattr.ModificationTime = modTime.Format("2006-01-02T15:04:05-0700") /* ISO8601 */
	return me.commitNewRevision(ctx, draft.nodeKR, draft.linkID, draft.revisionID, manifest, *xattr)
}

type uploadDraft struct {
	linkID     string
	revisionID string
	sessionKey *crypto.SessionKey
	nodeKR     *crypto.KeyRing
	verifier   *verificationData
}

func (me *Extension) createUploadDraft(ctx context.Context, parentLink *proton.Link, fileName string, modTime time.Time, mimeType string) (*uploadDraft, error) {
	payload, err := me.prepareFileCreation(ctx, parentLink, fileName)
	if err != nil {
		return nil, err
	}

	sessionKey, err := crypto.GenerateSessionKey()
	if err != nil {
		return nil, err
	}
	sessionKeyEnc, err := payload.NodeKR.EncryptSessionKey(sessionKey)
	if err != nil {
		return nil, err
	}
	sessionKeySignature, err := signMessage(crypto.NewPlainMessage(sessionKey.Key), payload.NodeKR)
	if err != nil {
		return nil, err
	}

	createFileReq := proton.CreateFileReq{
		ParentLinkID:              parentLink.LinkID,
		Name:                      payload.NameEnc,
		Hash:                      payload.NameHash,
		MIMEType:                  mimeType,
		ContentKeyPacket:          base64.StdEncoding.EncodeToString(sessionKeyEnc),
		ContentKeyPacketSignature: sessionKeySignature,
		NodeKey:                   payload.Key,
		NodePassphrase:            payload.Passphrase,
		NodePassphraseSignature:   payload.PassphraseSignature,
		SignatureAddress:          me.MainShare.Share.Creator,
	}

	first := true
retry:
	res, err := me.client.CreateFile(ctx, me.getShareID(), createFileReq)
	linkID := res.ID
	revisionID := res.RevisionID
	if err != nil {
		if apiErr, ok := errors.AsType[*proton.APIError](err); !ok || apiErr.Code != 2500 || !first {
			return nil, err
		}
		first = false

		link, err := me.findInFolder(ctx, parentLink, fileName, proton.LinkStateActive)
		if err != nil {
			return nil, err
		}

		if link == nil {
			link, err = me.findInFolder(ctx, parentLink, fileName, proton.LinkStateDraft)
			if err != nil {
				return nil, err
			}
		}

		if link == nil {
			// we have a real problem here (unless the assumption is wrong)
			// since we can't create a new file AND we can't locate a file with active/draft revision in it
			return nil, fmt.Errorf("can't locate revision for file: %w", err)
		}

		linkID = link.LinkID

		var submitAgain bool
		revisionID, submitAgain, err = me.handleRevisionConflict(ctx, link)
		if err != nil {
			return nil, err
		}
		if submitAgain {
			goto retry
		}

		parentNodeKR, err := me.getLinkKRByID(ctx, link.ParentLinkID)
		if err != nil {
			return nil, err
		}
		sessionKey, payload.NodeKR, err = me.getSessionKey(link, parentNodeKR)
		if err != nil {
			return nil, err
		}
	}

	verification, err := me.client.VerifyRevision(ctx, me.getShareID(), linkID, revisionID)
	if err != nil {
		return nil, err
	}
	encSessionKey, err := base64.StdEncoding.DecodeString(verification.ContentKeyPacket)
	if err != nil {
		return nil, err
	}
	verifierSessionKey, err := payload.NodeKR.DecryptSessionKey(encSessionKey)
	if err != nil {
		return nil, err
	}
	verificationCode, err := base64.StdEncoding.DecodeString(verification.VerificationCode)
	if err != nil {
		return nil, err
	}

	return &uploadDraft{
		linkID:     linkID,
		revisionID: revisionID,
		sessionKey: sessionKey,
		nodeKR:     payload.NodeKR,
		verifier: &verificationData{
			sessionKey:       verifierSessionKey,
			verificationCode: verificationCode,
		},
	}, nil
}

type verificationData struct {
	sessionKey       *crypto.SessionKey
	verificationCode []byte
}

func (verifier *verificationData) verifyBlock(encData []byte) ([]byte, error) {
	_, err := verifier.sessionKey.Decrypt(encData)
	if err != nil {
		return nil, fmt.Errorf("verification failed: %w", err)
	}

	result := make([]byte, len(verifier.verificationCode))
	for i, verCodeByte := range verifier.verificationCode {
		var encDataByte byte
		if i < len(encData) {
			encDataByte = encData[i]
		}
		result[i] = verCodeByte ^ encDataByte
	}
	return result, nil
}

func (me *Extension) uploadBlocks(ctx context.Context, draft *uploadDraft, file io.Reader) ([]byte, *revisionXAttr, error) {
	const UploadBlockSize = 4 * 1024 * 1024 // 4 MB

	pendingBlocks := &batchUploader{
		ext:        me,
		linkID:     draft.linkID,
		revisionID: draft.revisionID,
	}

	shouldContinue := true
	totalFileSize := int64(0)
	sha1Digest := sha1.New()
	blockSizes := make([]int64, 0)
	manifest := bytes.NewBuffer(nil)
	for i := 1; shouldContinue; i += 1 {
		data := make([]byte, UploadBlockSize) // FIXME: get block size from the server config instead of hardcoding it
		readBytes, err := io.ReadFull(file, data)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				if readBytes == 0 {
					break
				}
				shouldContinue = false
			} else {
				return nil, nil, err
			}
		}

		data = data[:readBytes]
		totalFileSize += int64(readBytes)
		sha1Digest.Write(data)
		blockSizes = append(blockSizes, int64(readBytes))

		clear := crypto.NewPlainMessage(data)
		encrypted, err := draft.sessionKey.Encrypt(clear)
		if err != nil {
			return nil, nil, err
		}
		encSignature, err := signEncryptedMessage(clear, me.MainShare.AddrKR, draft.nodeKR)
		if err != nil {
			return nil, nil, err
		}

		hashSized := sha256.Sum256(encrypted)
		hash := hashSized[:]
		manifest.Write(hash)

		verificationToken, err := draft.verifier.verifyBlock(encrypted)
		if err != nil {
			return nil, nil, err
		}

		pendingBlocks.batch = append(pendingBlocks.batch, pendingBlock{
			encData: encrypted,
			info: proton.BlockUploadInfo{
				Index:        i, // iOS drive: BE starts with 1
				Size:         int64(len(encrypted)),
				EncSignature: encSignature,
				Hash:         base64.StdEncoding.EncodeToString(hash),
				Verifier: proton.BlockUploadInfoVerifier{
					Token: base64.StdEncoding.EncodeToString(verificationToken),
				},
			},
		})

		err = pendingBlocks.flush(ctx, false)
		if err != nil {
			return nil, nil, err
		}
	}

	err := pendingBlocks.flush(ctx, true)
	if err != nil {
		return nil, nil, err
	}

	return manifest.Bytes(), &revisionXAttr{
		Size:       totalFileSize,
		BlockSizes: blockSizes,
		Digests: map[string]string{
			"SHA1": hex.EncodeToString(sha1Digest.Sum(nil)),
		},
	}, nil
}

type pendingBlock struct {
	encData []byte
	info    proton.BlockUploadInfo
}

type batchUploader struct {
	ext        *Extension
	linkID     string
	revisionID string
	batch      []pendingBlock
}

func (me *batchUploader) flush(ctx context.Context, flush bool) error {
	const (
		minimumBatchSize  = 8
		concurrentUploads = 20
	)

	if len(me.batch) == 0 || (!flush && len(me.batch) < minimumBatchSize) {
		return nil
	}

	blockList := make([]proton.BlockUploadInfo, 0, len(me.batch))
	for _, block := range me.batch {
		blockList = append(blockList, block.info)
	}
	res, err := me.ext.client.RequestBlockUpload(ctx, proton.BlockUploadReq{
		AddressID:  me.ext.MainShare.Share.AddressID,
		ShareID:    me.ext.getShareID(),
		LinkID:     me.linkID,
		RevisionID: me.revisionID,
		BlockList:  blockList,
	})
	if err != nil {
		return err
	}

	sem := semaphore.NewWeighted(concurrentUploads)

	g := new(errgroup.Group)
	for i := range res {
		block := res[i]
		data := me.batch[i].encData
		g.Go(func() error {
			sem.Acquire(ctx, 1)
			defer sem.Release(1)

			return me.ext.client.UploadBlock(ctx, block.BareURL, block.Token, resty.NewByteMultipartStream(data))
		})
	}

	me.batch = me.batch[:0]

	return g.Wait()
}

type revisionXAttr struct {
	ModificationTime string
	Size             int64
	BlockSizes       []int64
	Digests          map[string]string
}

type revisionXAttrWrapper struct {
	Common revisionXAttr
}

func (me *Extension) commitNewRevision(ctx context.Context, nodeKR *crypto.KeyRing, linkID, revisionID string, manifest []byte, xattr revisionXAttr) error {
	manifestSignature, err := signMessage(crypto.NewPlainMessage(manifest), me.MainShare.AddrKR)
	if err != nil {
		return err
	}

	xattrJSON, err := json.Marshal(revisionXAttrWrapper{
		Common: xattr,
	})
	if err != nil {
		return err
	}

	xattrJSONEnc, err := encryptMessage(crypto.NewPlainMessage(xattrJSON), me.MainShare.AddrKR, nodeKR)
	if err != nil {
		return err
	}

	return me.client.UpdateRevision(ctx, me.getShareID(), linkID, revisionID, proton.UpdateRevisionReq{
		ManifestSignature: manifestSignature,
		SignatureAddress:  me.MainShare.Share.Creator,
		XAttr:             xattrJSONEnc,
	})
}
