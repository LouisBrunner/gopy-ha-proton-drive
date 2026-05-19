package protonx

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type Share struct {
	Share      *proton.Share
	Keyring    *crypto.KeyRing
	AddrKR     *crypto.KeyRing
	RootFolder *proton.Link
}

func (me *Extension) FetchShare(ctx context.Context, shareID string) (*Share, error) {
	share, err := me.client.GetShare(ctx, shareID)
	if err != nil {
		return nil, err
	}

	pk, _, err := me.client.GetPublicKeys(ctx, share.Creator)
	if err != nil {
		return nil, fmt.Errorf("failed to get public keys for address %q: %w", share.Creator, err)
	}
	creatorKR, err := pk.GetKeyRing()
	if err != nil {
		return nil, fmt.Errorf("failed to get keyring for address %q: %w", share.Creator, err)
	}
	me.emailKRs[share.Creator] = creatorKR

	shareAddrKR, found := me.addrKRs[share.AddressID]
	if !found {
		return nil, fmt.Errorf("failed to get share address keyring for %q", shareID)
	}
	err = concatKR(shareAddrKR, creatorKR)
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

	return &Share{
		Share:      &share,
		Keyring:    keyring,
		AddrKR:     shareAddrKR,
		RootFolder: &folder,
	}, nil
}
