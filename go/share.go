package proton

import (
	"context"
	"fmt"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
)

type Share struct {
	Name    string
	ShareID string
}

func (me *Client) ListShares(ctx context.Context) ([]Share, error) {
	shares, err := me.client.ListShares(ctx, true)
	if err != nil {
		return nil, err
	}

	fshares := make([]Share, len(shares))
	for i, shareMeta := range shares {
		var name string

		if shareMeta.Type != proton.ShareTypeMain {
			shareData, err := me.fetchShare(ctx, shareMeta.ShareID)
			if err != nil {
				return nil, fmt.Errorf("failed for share %d: %w", i, err)
			}
			name, err = shareData.rootFolder.GetName(shareData.keyring, shareData.addrKeyring)
			if err != nil {
				return nil, fmt.Errorf("failed to get name for share %d: %w", i, err)
			}
		}

		switch shareMeta.Type {
		case proton.ShareTypeMain:
			name = "My files"
		case proton.ShareTypeStandard:
			name += fmt.Sprintf(" (Shared by %s)", shareMeta.Creator)
		case proton.ShareTypeDevice:
			name += " (Device)"
		}
		fshares[i] = Share{
			Name:    name,
			ShareID: shareMeta.ShareID,
		}
	}
	return fshares, nil
}

func (me *Client) SelectShare(ctx context.Context, shareID string) error {
	shareData, err := me.fetchShare(ctx, shareID)
	if err != nil {
		return err
	}

	me.drive.MainShare = shareData.share
	me.drive.MainShareKR = shareData.keyring
	me.drive.RootLink = shareData.rootFolder

	return nil
}

func (me *Client) concatKR(kr *crypto.KeyRing, keyrings ...*crypto.KeyRing) error {
	for _, keyring := range keyrings {
		for _, key := range keyring.GetKeys() {
			err := kr.AddKey(key)
			if err != nil {
				return fmt.Errorf("failed to add key to keyring: %w", err)
			}
		}
	}
	return nil
}

type shareData struct {
	share       *proton.Share
	keyring     *crypto.KeyRing
	addrKeyring *crypto.KeyRing
	rootFolder  *proton.Link
}

func (me *Client) fetchShare(ctx context.Context, shareID string) (*shareData, error) {
	share, err := me.client.GetShare(ctx, shareID)
	if err != nil {
		return nil, err
	}

	creatorKR, err := me.drive.AddKeyRingForEmail(ctx, share.Creator)
	if err != nil {
		return nil, fmt.Errorf("failed to add creator address to keyring for share %q: %w", shareID, err)
	}

	shareAddrKR, found := me.drive.GetKeyRingForAddressID(share.AddressID)
	if !found {
		return nil, fmt.Errorf("failed to get share address keyring for %q", shareID)
	}
	err = me.concatKR(shareAddrKR, creatorKR)
	if err != nil {
		return nil, fmt.Errorf("failed to concat keyrings for share %q: %w", shareID, err)
	}

	keyring, err := share.GetKeyRing(shareAddrKR)
	if err != nil {
		return nil, fmt.Errorf("failed to get keyring for share %q: %w", shareID, err)
	}

	folder, err := me.client.GetLink(ctx, share.ShareID, share.LinkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder for share %q: %w", shareID, err)
	}

	return &shareData{
		share:       &share,
		keyring:     keyring,
		addrKeyring: shareAddrKR,
		rootFolder:  &folder,
	}, nil
}
