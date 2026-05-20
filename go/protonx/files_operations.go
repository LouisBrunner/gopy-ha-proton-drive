package protonx

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
)

type DirEntry struct {
	Link     *proton.Link
	Name     string
	IsFolder bool
}

func (me *Extension) ListDirectory(ctx context.Context, folderLinkID string) ([]*DirEntry, error) {
	ret := make([]*DirEntry, 0)

	folderLink, err := me.GetLink(ctx, folderLinkID)
	if err != nil {
		return nil, err
	}

	if folderLink.State != proton.LinkStateActive {
		return ret, nil
	}

	childrenLinks, err := me.client.ListChildren(ctx, me.getShareID(), folderLink.LinkID, true)
	if err != nil {
		return nil, err
	}

	if childrenLinks != nil {
		folderLinkKR, err := me.getLinkKR(ctx, folderLink)
		if err != nil {
			return nil, err
		}

		for i := range childrenLinks {
			if childrenLinks[i].State != proton.LinkStateActive {
				continue
			}

			signatureVerificationKR, err := me.buildKeyring([]string{childrenLinks[i].NameSignatureEmail, childrenLinks[i].SignatureEmail})
			if err != nil {
				return nil, err
			}
			name, err := childrenLinks[i].GetName(folderLinkKR, signatureVerificationKR)
			if err != nil {
				return nil, err
			}
			ret = append(ret, &DirEntry{
				Link:     &childrenLinks[i],
				Name:     name,
				IsFolder: childrenLinks[i].Type == proton.LinkTypeFolder,
			})
		}
	}
	return ret, nil
}

func (me *Extension) MoveFileToTrash(ctx context.Context, link *proton.Link) error {
	if link.Type != proton.LinkTypeFile {
		return fmt.Errorf("link type must be file")
	}

	err := me.client.TrashChildren(ctx, me.getShareID(), link.ParentLinkID, link.LinkID)
	if err != nil {
		return err
	}

	me.cache.Remove(link.LinkID, true)

	return nil
}

func (me *Extension) CreateFolder(ctx context.Context, parentLink *proton.Link, folderName string) (string, error) {
	payload, err := me.prepareFileCreation(ctx, parentLink, folderName)
	if err != nil {
		return "", err
	}

	nodeHashKey, err := encryptRandom(payload.NodeKR)
	if err != nil {
		return "", err
	}

	res, err := me.client.CreateFolder(ctx, me.getShareID(), proton.CreateFolderReq{
		ParentLinkID:            parentLink.LinkID,
		Name:                    payload.NameEnc,
		Hash:                    payload.NameHash,
		NodeHashKey:             nodeHashKey,
		NodeKey:                 payload.Key,
		NodePassphrase:          payload.Passphrase,
		NodePassphraseSignature: payload.PassphraseSignature,
		SignatureAddress:        me.creatorAddress,
	})
	if err != nil {
		return "", err
	}

	return res.ID, nil
}
