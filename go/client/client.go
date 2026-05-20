package client

import (
	"context"
	"fmt"

	"github.com/LouisBrunner/gopy-ha-proton-drive/go/protonx"
	"github.com/ProtonMail/go-proton-api"
	"github.com/sirupsen/logrus"
)

type Client struct {
	manager              *proton.Manager
	client               *proton.Client
	extension            *protonx.Extension
	logger               *logrus.Logger
	uploadTries          int
	uploadChunkSizeBytes uint64
}

type OnAuthChange func(creds Credentials)

type Options struct {
	Logger               *logrus.Logger
	Credentials          Credentials
	OnAuthChange         OnAuthChange
	MaxUploadTries       int
	UploadChunkSizeBytes uint64
	ShareID              string
}

func New(ctx context.Context, opts Options) (*Client, error) {
	manager := NewManager(opts.Logger)

	client := manager.NewClient(opts.Credentials.UID, opts.Credentials.AccessToken, opts.Credentials.RefreshToken)
	client.AddAuthHandler(func(a proton.Auth) {
		if opts.OnAuthChange == nil {
			return
		}
		opts.OnAuthChange(Credentials{
			UID:           a.UID,
			AccessToken:   a.AccessToken,
			RefreshToken:  a.RefreshToken,
			SaltedKeyPass: opts.Credentials.SaltedKeyPass,
		})
	})

	extension, err := protonx.New(ctx, client, opts.Credentials.SaltedKeyPass)
	if err != nil {
		return nil, err
	}

	shareID := opts.ShareID
	if shareID == "" {
		volumes, err := client.ListVolumes(ctx)
		if err != nil {
			return nil, err
		}

		for i := range volumes {
			if volumes[i].State == proton.VolumeStateActive {
				shareID = volumes[i].Share.ShareID
				break
			}
		}

		if shareID == "" {
			return nil, fmt.Errorf("no active volume found")
		}
	}

	extension.MainShare, err = extension.FetchShare(ctx, shareID)
	if err != nil {
		return nil, err
	}

	if opts.ShareID == "" {
		allShares, err := client.ListShares(ctx, true)
		if err != nil {
			return nil, fmt.Errorf("failed to list shares for integrity check: %w", err)
		}
		mainShareValid := false
		for _, s := range allShares {
			if s.ShareID == extension.MainShare.Share.ShareID &&
				s.LinkID == extension.MainShare.Share.LinkID &&
				s.Flags == proton.PrimaryShare &&
				s.Type == proton.ShareTypeMain {
				mainShareValid = true
				break
			}
		}
		if !mainShareValid {
			return nil, fmt.Errorf("main share %q failed integrity check: not a primary main share", opts.ShareID)
		}
	}

	return &Client{
		manager:              manager,
		client:               client,
		extension:            extension,
		logger:               opts.Logger,
		uploadTries:          opts.MaxUploadTries,
		uploadChunkSizeBytes: opts.UploadChunkSizeBytes,
	}, nil
}
