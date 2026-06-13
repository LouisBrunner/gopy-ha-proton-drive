package protonx

import (
	"fmt"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

func concatKR(kr *crypto.KeyRing, keyrings ...*crypto.KeyRing) error {
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

func decryptKeyring(kr, addrKR *crypto.KeyRing, key, passphrase, passphraseSignature string) (*crypto.KeyRing, error) {
	enc, err := crypto.NewPGPMessageFromArmored(passphrase)
	if err != nil {
		return nil, err
	}

	dec, err := kr.Decrypt(enc, nil, crypto.GetUnixTime())
	if err != nil {
		return nil, err
	}

	sig, err := crypto.NewPGPSignatureFromArmored(passphraseSignature)
	if err != nil {
		return nil, err
	}

	if err := addrKR.VerifyDetached(dec, sig, crypto.GetUnixTime()); err != nil {
		return nil, err
	}

	lockedKey, err := crypto.NewKeyFromArmored(key)
	if err != nil {
		return nil, err
	}

	unlockedKey, err := lockedKey.Unlock(dec.GetBinary())
	if err != nil {
		return nil, err
	}

	return crypto.NewKeyRing(unlockedKey)
}

func (me *Extension) buildKeyring(emailAddresses []string, verificationAddrKRs ...*crypto.KeyRing) (*crypto.KeyRing, error) {
	ret, err := crypto.NewKeyRing(nil)
	if err != nil {
		return nil, err
	}

	for _, emailAddress := range emailAddresses {
		if addr, ok := me.emailToAddress[emailAddress]; ok {
			if err := concatKR(ret, me.addrKRs[addr.ID]); err != nil {
				return nil, err
			}
		}
		if emailKR, ok := me.emailKRs[emailAddress]; ok {
			if err := concatKR(ret, emailKR); err != nil {
				return nil, err
			}
		}
	}

	if err := concatKR(ret, verificationAddrKRs...); err != nil {
		return nil, err
	}

	if ret.CountEntities() == 0 {
		return nil, fmt.Errorf("no keyring for signature verification")
	}
	return ret, nil
}
