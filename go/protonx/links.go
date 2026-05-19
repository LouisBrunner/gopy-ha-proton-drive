package protonx

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

func (me *Extension) GetLink(ctx context.Context, linkID string) (*proton.Link, error) {
	if linkID == "" {
		return nil, fmt.Errorf("invalid linkID")
	}

	// attempt to get from cache first
	if data := me.cache.Get(linkID); data != nil && data.Link != nil {
		return data.Link, nil
	}

	// no cached data, fetch
	link, err := me.client.GetLink(ctx, me.getShareID(), linkID)
	if err != nil {
		return nil, err
	}

	// populate cache
	me.cache.Insert(linkID, &link, nil)
	return &link, nil
}

func (me *Extension) getLinkKRByID(ctx context.Context, linkID string) (*crypto.KeyRing, error) {
	if linkID == "" {
		return me.MainShare.Keyring, nil
	}

	link, err := me.GetLink(ctx, linkID)
	if err != nil {
		return nil, err
	}

	return me.getLinkKR(ctx, link)
}

func (me *Extension) getLinkKR(ctx context.Context, link *proton.Link) (*crypto.KeyRing, error) {
	if link == nil {
		return nil, fmt.Errorf("invalid linkID")
	}

	// attempt to get from cache first
	if data := me.cache.Get(link.LinkID); data != nil && data.KR != nil {
		return data.KR, nil
	}

	// no cached data, fetch
	kr, err := me.fetchKRForLink(ctx, link)
	if err != nil {
		return nil, err
	}

	// populate cache
	me.cache.Insert(link.LinkID, link, kr)
	return kr, nil
}

func (me *Extension) fetchKRForLink(ctx context.Context, link *proton.Link) (*crypto.KeyRing, error) {
	parentNodeKR, err := me.getLinkKRByID(ctx, link.ParentLinkID)
	if err != nil {
		return nil, err
	}
	signatureVerificationKR, err := me.buildKeyring([]string{link.SignatureEmail})
	if err != nil {
		return nil, err
	}
	return link.GetKeyRing(parentNodeKR, signatureVerificationKR)
}
