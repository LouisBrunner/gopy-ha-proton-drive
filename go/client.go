package proton

import (
	"context"

	proton_api_bridge "github.com/henrybear327/Proton-API-Bridge"
	"github.com/henrybear327/Proton-API-Bridge/common"
	"github.com/henrybear327/go-proton-api"
	"github.com/sirupsen/logrus"
)

type Client struct {
	drive                *proton_api_bridge.ProtonDrive
	client               *proton.Client
	logger               *logrus.Logger
	uploadTries          int
	uploadChunkSizeBytes uint64
}

type Credentials common.ProtonDriveCredential

func getConfig() *common.Config {
	config := proton_api_bridge.NewDefaultConfig()
	config.AppVersion = "Other"
	config.UserAgent = "HomeAssistant"
	config.ReplaceExistingDraft = true
	config.EnableCaching = true
	return config
}

func Login(ctx context.Context, username, password, mfa string) (*Credentials, error) {
	config := getConfig()
	config.FirstLoginCredential.Username = username
	config.FirstLoginCredential.Password = password
	config.FirstLoginCredential.TwoFA = mfa

	_, auth, err := proton_api_bridge.NewProtonDrive(ctx, config, func(a proton.Auth) {}, func() {})
	if err != nil {
		return nil, err
	}

	return (*Credentials)(auth), nil
}

type OnAuthChange func(creds Credentials)

type ClientOptions struct {
	Logger               *logrus.Logger
	Credentials          Credentials
	OnAuthChange         OnAuthChange
	MaxUploadTries       int
	UploadChunkSizeBytes uint64
}

func NewClient(ctx context.Context, opts ClientOptions) (*Client, error) {
	config := getConfig()
	config.UseReusableLogin = true
	config.ReusableCredential.UID = opts.Credentials.UID
	config.ReusableCredential.AccessToken = opts.Credentials.AccessToken
	config.ReusableCredential.RefreshToken = opts.Credentials.RefreshToken
	config.ReusableCredential.SaltedKeyPass = opts.Credentials.SaltedKeyPass

	drive, _, err := proton_api_bridge.NewProtonDrive(ctx, config, func(a proton.Auth) {
		config.ReusableCredential.UID = a.UID
		config.ReusableCredential.AccessToken = a.AccessToken
		config.ReusableCredential.RefreshToken = a.RefreshToken
		if opts.OnAuthChange == nil {
			return
		}
		opts.OnAuthChange(Credentials{
			UID:           a.UID,
			AccessToken:   a.AccessToken,
			RefreshToken:  a.RefreshToken,
			SaltedKeyPass: opts.Credentials.SaltedKeyPass,
		})
	}, func() {})
	if err != nil {
		return nil, err
	}

	realClient := proton.New(
		proton.WithAppVersion(config.AppVersion),
		proton.WithUserAgent(config.UserAgent),
	).NewClient(config.ReusableCredential.UID, config.ReusableCredential.AccessToken, config.ReusableCredential.RefreshToken)

	return &Client{
		drive:                drive,
		client:               realClient,
		logger:               opts.Logger,
		uploadTries:          opts.MaxUploadTries,
		uploadChunkSizeBytes: opts.UploadChunkSizeBytes,
	}, nil
}
