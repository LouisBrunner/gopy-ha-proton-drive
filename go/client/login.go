package client

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
)

type LoginOptions struct {
	Username        string
	Password        string
	TwoFA           string
	MailboxPassword string
	CaptchaToken    string
}

var (
	ErrMFARequired         = fmt.Errorf("mfa code is required")
	ErrMailboxPassRequired = fmt.Errorf("mailbox password is required")
)

func Login(ctx context.Context, manager *proton.Manager, opts LoginOptions) (*Credentials, error) {
	var c *proton.Client
	var auth proton.Auth
	var err error
	if opts.CaptchaToken != "" {
		c, auth, err = manager.NewClientWithLoginWithHVToken(ctx, opts.Username, []byte(opts.Password), &proton.APIHVDetails{
			Methods: []string{"captcha"},
			Token:   opts.CaptchaToken,
		})
	} else {
		c, auth, err = manager.NewClientWithLogin(ctx, opts.Username, []byte(opts.Password))
	}
	if err != nil {
		// TODO: catch captcha error
		return nil, err
	}

	if auth.TwoFA.Enabled&proton.HasTOTP != 0 {
		if opts.TwoFA != "" {
			err := c.Auth2FA(ctx, proton.Auth2FAReq{
				TwoFactorCode: opts.TwoFA,
			})
			if err != nil {
				return nil, err
			}
		} else {
			return nil, ErrMFARequired
		}
	}

	var keyPass []byte
	if auth.PasswordMode == proton.TwoPasswordMode {
		if opts.MailboxPassword != "" {
			keyPass = []byte(opts.MailboxPassword)
		} else {
			return nil, ErrMailboxPassRequired
		}
	} else {
		keyPass = []byte(opts.Password)
	}

	user, err := c.GetUser(ctx)
	if err != nil {
		return nil, err
	}
	salts, err := c.GetSalts(ctx)
	if err != nil {
		return nil, err
	}
	saltedKeyPass, err := salts.SaltForKey(keyPass, user.Keys.Primary().ID)
	if err != nil {
		return nil, err
	}

	return &Credentials{
		UID:           auth.UID,
		AccessToken:   auth.AccessToken,
		RefreshToken:  auth.RefreshToken,
		SaltedKeyPass: base64.StdEncoding.EncodeToString(saltedKeyPass),
	}, nil
}

type Credentials struct {
	UID           string `json:"uid"`
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	SaltedKeyPass string `json:"salted_key_pass"`
}
