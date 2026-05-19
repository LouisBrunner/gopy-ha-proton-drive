package protonx

import (
	"context"
	"errors"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

func getHashKey(link *proton.Link, parentNodeKey, addrKRs *crypto.KeyRing) ([]byte, error) {
	if link.Type != proton.LinkTypeFolder {
		return nil, errors.New("link is not a folder")
	}

	enc, err := crypto.NewPGPMessageFromArmored(link.FolderProperties.NodeHashKey)
	if err != nil {
		return nil, err
	}

	_, ok := enc.GetSignatureKeyIDs()
	var dec *crypto.PlainMessage
	if ok {
		dec, err = parentNodeKey.Decrypt(enc, addrKRs, crypto.GetUnixTime())
		if err != nil {
			return nil, err
		}
	} else {
		dec, err = parentNodeKey.Decrypt(enc, nil, 0)
		if err != nil {
			return nil, err
		}
	}

	return dec.GetBinary(), nil
}

func (me *Extension) getSessionKey(link *proton.Link, parentNodeKR *crypto.KeyRing) (*crypto.SessionKey, *crypto.KeyRing, error) {
	signatureVerificationKR, err := me.buildKeyring([]string{link.SignatureEmail})
	if err != nil {
		return nil, nil, err
	}
	nodeKR, err := link.GetKeyRing(parentNodeKR, signatureVerificationKR)
	if err != nil {
		return nil, nil, err
	}
	sessionKey, err := link.GetSessionKey(nodeKR)
	if err != nil {
		return nil, nil, err
	}
	return sessionKey, nodeKR, nil
}

func (me *Extension) calcNameHash(parentLink *proton.Link, parentNodeKR *crypto.KeyRing, fileName string) (string, error) {
	signatureVerificationKR, err := me.buildKeyring([]string{parentLink.SignatureEmail}, parentNodeKR)
	if err != nil {
		return "", err
	}
	parentHashKey, err := getHashKey(parentLink, parentNodeKR, signatureVerificationKR)
	if err != nil {
		return "", err
	}
	return doHMAC(fileName, parentHashKey)
}

type createFilePayload struct {
	NameEnc             string
	NameHash            string
	Key                 string
	Passphrase          string
	PassphraseSignature string
	NodeKR              *crypto.KeyRing
}

func (me *Extension) prepareFileCreation(ctx context.Context, parentLink *proton.Link, fileName string) (*createFilePayload, error) {
	parentNodeKR, err := me.getLinkKR(ctx, parentLink)
	if err != nil {
		return nil, err
	}

	name, err := encryptString(fileName, me.MainShare.AddrKR, parentNodeKR)
	if err != nil {
		return nil, err
	}

	hash, err := me.calcNameHash(parentLink, parentNodeKR, fileName)
	if err != nil {
		return nil, err
	}

	key, passphrase, passphraseSig, err := generateKeyAndSignedPassphrase(me.MainShare.AddrKR, parentNodeKR)
	if err != nil {
		return nil, err
	}

	nodeKR, err := decryptKeyring(parentNodeKR, me.MainShare.AddrKR, key, passphrase, passphraseSig)
	if err != nil {
		return nil, err
	}

	return &createFilePayload{
		NameEnc:             name,
		NameHash:            hash,
		Key:                 key,
		Passphrase:          passphrase,
		PassphraseSignature: passphraseSig,
		NodeKR:              nodeKR,
	}, nil
}

func (me *Extension) findInFolder(ctx context.Context, folderLink *proton.Link, targetName string, targetState proton.LinkState) (*proton.Link, error) {
	if folderLink.Type != proton.LinkTypeFolder {
		return nil, fmt.Errorf("link must be a folder")
	}

	if folderLink.State != proton.LinkStateActive {
		// we only search in the active folders
		return nil, nil
	}

	folderLinkKR, err := me.fetchKRForLink(ctx, folderLink)
	if err != nil {
		return nil, err
	}
	targetNameHash, err := me.calcNameHash(folderLink, folderLinkKR, targetName)
	if err != nil {
		return nil, err
	}

	// more efficient than linear scan to just do existence check
	res, err := me.client.CheckAvailableHashes(ctx, me.getShareID(), folderLink.LinkID, proton.CheckAvailableHashesReq{
		Hashes: []string{targetNameHash},
	})
	if err != nil {
		return nil, err
	}

	if len(res.AvailableHashes) == 1 {
		// name isn't taken == name doesn't exist
		return nil, nil
	}

	childrenLinks, err := me.client.ListChildren(ctx, me.getShareID(), folderLink.LinkID, true)
	if err != nil {
		return nil, err
	}

	for _, childLink := range childrenLinks {
		if childLink.State != targetState {
			continue
		}

		if childLink.Type == proton.LinkTypeFile && childLink.Hash == targetNameHash {
			return &childLink, nil
		}
	}

	return nil, nil
}

func (me *Extension) handleRevisionConflict(ctx context.Context, link *proton.Link) (string, bool, error) {
	linkID := link.LinkID

	draftRevision, err := me.getRevisionsFor(ctx, link, proton.RevisionStateDraft)
	if err != nil {
		return "", false, err
	}

	// if we have a draft revision, we recreate a draft
	// if we have no draft revision, then we can create a new draft revision directly (there is a restriction of 1 draft revision per file)

	if len(draftRevision) > 0 {
		// delete the draft revision (will fail if the file only have a draft but no active revisions)
		if link.State == proton.LinkStateDraft {
			// delete the link (skipping trash, otherwise it won't work)
			err = me.client.DeleteChildren(ctx, me.getShareID(), link.ParentLinkID, linkID)
			if err != nil {
				return "", false, err
			}

			return "", true, nil
		}

		// delete the draft revision
		err = me.client.DeleteRevision(ctx, me.getShareID(), linkID, draftRevision[0].ID)
		if err != nil {
			return "", false, err
		}
	}

	// create a new revision
	newRevision, err := me.client.CreateRevision(ctx, me.getShareID(), linkID)
	if err != nil {
		return "", false, err
	}

	return newRevision.ID, false, nil
}

func (me *Extension) getRevisionsFor(ctx context.Context, link *proton.Link, revisionType proton.RevisionState) ([]*proton.RevisionMetadata, error) {
	revisions, err := me.client.ListRevisions(ctx, me.getShareID(), link.LinkID)
	if err != nil {
		return nil, err
	}

	ret := make([]*proton.RevisionMetadata, 0)
	for i := range revisions {
		if revisions[i].State == revisionType {
			ret = append(ret, &revisions[i])
		}
	}

	return ret, nil
}
