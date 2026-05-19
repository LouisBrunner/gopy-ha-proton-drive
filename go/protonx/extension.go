package protonx

import (
	"context"
	"encoding/base64"

	"github.com/LouisBrunner/gopy-ha-proton-drive/go/protonx/internal/cache"
	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type Extension struct {
	client         *proton.Client
	cache          *cache.Cache
	addrKRs        map[string]*crypto.KeyRing
	emailKRs       map[string]*crypto.KeyRing
	emailToAddress map[string]proton.Address
	creator        string
	MainShare      *Share
}

func New(ctx context.Context, client *proton.Client, saltedKeyPassB64 string) (*Extension, error) {
	user, err := client.GetUser(ctx)
	if err != nil {
		return nil, err
	}

	addresses, err := client.GetAddresses(ctx)
	if err != nil {
		return nil, err
	}

	saltedKeyPass, err := base64.StdEncoding.DecodeString(saltedKeyPassB64)
	if err != nil {
		return nil, err
	}

	_, addrKRs, err := proton.Unlock(user, addresses, saltedKeyPass, nil)
	if err != nil {
		return nil, err
	}

	emailToAddress := make(map[string]proton.Address, len(addresses))
	for _, addr := range addresses {
		emailToAddress[addr.Email] = addr
	}

	return &Extension{
		client:         client,
		cache:          cache.New(),
		addrKRs:        addrKRs,
		emailKRs:       make(map[string]*crypto.KeyRing),
		emailToAddress: emailToAddress,
		creator:        user.Email,
	}, nil
}

func (me *Extension) getShareID() string {
	return me.MainShare.Share.ShareID
}
