package client

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
)

type Share struct {
	Name    string `json:"name"`
	ShareID string `json:"share_id"`
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
			shareData, err := me.extension.FetchShare(ctx, shareMeta.ShareID)
			if err != nil {
				return nil, fmt.Errorf("failed for share %d: %w", i, err)
			}
			name, err = shareData.RootFolder.GetName(shareData.Keyring, shareData.AddrKR)
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
