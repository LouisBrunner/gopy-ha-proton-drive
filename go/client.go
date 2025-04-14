package proton

import (
	"context"

	proton_api_bridge "github.com/henrybear327/Proton-API-Bridge"
	"github.com/henrybear327/Proton-API-Bridge/common"
	"github.com/henrybear327/go-proton-api"
)

type Client struct {
	drive  *proton_api_bridge.ProtonDrive
	client *proton.Client
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

func Login(username, password, mfa string) (*Credentials, error) {
	config := getConfig()
	config.FirstLoginCredential.Username = username
	config.FirstLoginCredential.Password = password
	config.FirstLoginCredential.TwoFA = mfa

	_, auth, err := proton_api_bridge.NewProtonDrive(context.Background(), config, func(a proton.Auth) {}, func() {})
	if err != nil {
		return nil, err
	}

	return (*Credentials)(auth), nil
}

func NewClient(creds Credentials, onAuthChange func(uid, accessToken, refreshToken string)) (*Client, error) {
	config := getConfig()
	config.UseReusableLogin = true
	config.ReusableCredential.UID = creds.UID
	config.ReusableCredential.AccessToken = creds.AccessToken
	config.ReusableCredential.RefreshToken = creds.RefreshToken
	config.ReusableCredential.SaltedKeyPass = creds.SaltedKeyPass

	drive, _, err := proton_api_bridge.NewProtonDrive(context.Background(), config, func(a proton.Auth) {
		config.ReusableCredential.UID = a.UID
		config.ReusableCredential.AccessToken = a.AccessToken
		config.ReusableCredential.RefreshToken = a.RefreshToken
		if onAuthChange == nil {
			return
		}
		onAuthChange(a.UID, a.AccessToken, a.RefreshToken)
	}, func() {})
	if err != nil {
		return nil, err
	}

	realClient := proton.New(
		proton.WithAppVersion(config.AppVersion),
		proton.WithUserAgent(config.UserAgent),
	).NewClient(config.ReusableCredential.UID, config.ReusableCredential.AccessToken, config.ReusableCredential.RefreshToken)

	return &Client{
		drive:  drive,
		client: realClient,
	}, nil
}
