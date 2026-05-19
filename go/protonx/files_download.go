package protonx

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type downloadReader struct {
	ext *Extension

	ctx context.Context

	link         *proton.Link
	data         *bytes.Buffer
	nodeKR       *crypto.KeyRing
	sessionKey   *crypto.SessionKey
	revision     *proton.Revision
	currentBlock int

	isEOF bool
}

func (me *downloadReader) Read(p []byte) (int, error) {
	if me.data == nil || me.data.Len() == 0 {
		buf, err := me.readBlock()
		if err != nil {
			return 0, err
		}
		me.data = buf

		if me.isEOF {
			return 0, io.EOF
		}
	}

	return me.data.Read(p)
}

func (me *downloadReader) Close() error {
	return nil
}

func (me *downloadReader) readBlock() (*bytes.Buffer, error) {
	if me.currentBlock == len(me.revision.Blocks) {
		me.isEOF = true
		return nil, nil
	}

	block := me.revision.Blocks[me.currentBlock]

	blockReader, err := me.ext.client.GetBlock(me.ctx, block.BareURL, block.Token)
	if err != nil {
		return nil, err
	}
	defer blockReader.Close()

	signatureVerificationKR, err := me.ext.buildKeyring([]string{me.link.SignatureEmail}, me.nodeKR)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(blockReader)
	if err != nil {
		return nil, err
	}
	if sha256Base64(data) != block.Hash {
		return nil, fmt.Errorf("downloaded block hash mismatch")
	}

	clear, err := decryptMessage(me.sessionKey, signatureVerificationKR, me.nodeKR, block.EncSignature, data)
	if err != nil {
		return nil, err
	}

	me.currentBlock += 1
	return bytes.NewBuffer(clear.GetBinary()), nil
}

func (me *Extension) DownloadFile(ctx context.Context, link *proton.Link) (io.ReadCloser, error) {
	if link.Type != proton.LinkTypeFile {
		return nil, fmt.Errorf("link type must be a file")
	}

	parentNodeKR, err := me.getLinkKRByID(ctx, link.ParentLinkID)
	if err != nil {
		return nil, err
	}

	sessionKey, nodeKR, err := me.getSessionKey(link, parentNodeKR)
	if err != nil {
		return nil, err
	}

	revision, err := me.getActiveRevision(ctx, link)
	if err != nil {
		return nil, err
	}

	return &downloadReader{
		ctx:        ctx,
		link:       link,
		data:       bytes.NewBuffer(nil),
		nodeKR:     nodeKR,
		sessionKey: sessionKey,
		revision:   revision,
	}, nil
}

func (me *Extension) getActiveRevision(ctx context.Context, link *proton.Link) (*proton.Revision, error) {
	if link == nil {
		return nil, fmt.Errorf("link must not be nil")
	}

	revisionsMetadata, err := me.getRevisionsFor(ctx, link, proton.RevisionStateActive)
	if err != nil {
		return nil, err
	}

	if len(revisionsMetadata) != 1 {
		return nil, fmt.Errorf("expected 1 active revision, got %d", len(revisionsMetadata))
	}

	revision, err := me.client.GetRevisionAllBlocks(ctx, me.getShareID(), link.LinkID, revisionsMetadata[0].ID)
	if err != nil {
		return nil, err
	}

	return &revision, nil
}
