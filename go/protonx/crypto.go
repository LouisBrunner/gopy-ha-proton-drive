package protonx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/helper"
)

func generateRandom() ([]byte, error) {
	return crypto.RandomToken(32)
}

func generatePassphrase() (string, error) {
	rand, err := generateRandom()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(rand), nil
}

func encryptRandom(nodeKR *crypto.KeyRing) (string, error) {
	rand, err := generateRandom()
	if err != nil {
		return "", err
	}
	return encryptMessage(crypto.NewPlainMessage(rand), nodeKR, nodeKR)
}

func encryptString(data string, addrKR, nodeKR *crypto.KeyRing) (string, error) {
	return encryptMessage(crypto.NewPlainMessageFromString(data), addrKR, nodeKR)
}

func encryptMessage(clear *crypto.PlainMessage, addrKR, nodeKR *crypto.KeyRing) (string, error) {
	enc, err := nodeKR.Encrypt(clear, addrKR)
	if err != nil {
		return "", err
	}
	return enc.GetArmored()
}

func signMessage(clear *crypto.PlainMessage, addrKR *crypto.KeyRing) (string, error) {
	sig, err := addrKR.SignDetached(clear)
	if err != nil {
		return "", err
	}
	return sig.GetArmored()
}

func signEncryptedMessage(clear *crypto.PlainMessage, addrKR, nodeKR *crypto.KeyRing) (string, error) {
	sig, err := addrKR.SignDetachedEncrypted(clear, nodeKR)
	if err != nil {
		return "", err
	}
	return sig.GetArmored()
}

func encryptSignMessage(clear *crypto.PlainMessage, addrKR, nodeKR *crypto.KeyRing) (string, string, error) {
	encrypted, err := encryptMessage(clear, nil, nodeKR)
	if err != nil {
		return "", "", err
	}

	sig, err := signMessage(clear, addrKR)
	if err != nil {
		return "", "", err
	}

	return encrypted, sig, nil
}

func decryptMessage(sessionKey *crypto.SessionKey, addrKR, nodeKR *crypto.KeyRing, encSignArm string, data []byte) (*crypto.PlainMessage, error) {
	clear, err := sessionKey.Decrypt(data)
	if err != nil {
		return nil, err
	}

	encSign, err := crypto.NewPGPMessageFromArmored(encSignArm)
	if err != nil {
		return nil, err
	}

	err = addrKR.VerifyDetachedEncrypted(clear, encSign, nodeKR, crypto.GetUnixTime())
	if err != nil {
		return nil, err
	}

	return clear, nil
}

func doHMAC(data string, hashKey []byte) (string, error) {
	mac := hmac.New(sha256.New, hashKey)
	_, err := mac.Write([]byte(data))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func sha256Base64(data []byte) string {
	hash := sha256.New()
	hash.Write(data)
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

func generateKeyAndPassphrase() (string, string, error) {
	passphrase, err := generatePassphrase()
	if err != nil {
		return "", "", err
	}

	// all hardcoded values from iOS drive
	key, err := helper.GenerateKey("Drive key", "noreply@protonmail.com", []byte(passphrase), "x25519", 0)
	if err != nil {
		return "", "", err
	}

	return key, passphrase, nil
}

func generateKeyAndSignedPassphrase(addrKR, nodeKR *crypto.KeyRing) (string, string, string, error) {
	key, passphrase, err := generateKeyAndPassphrase()
	if err != nil {
		return "", "", "", err
	}

	passphraseEnc, passphraseSig, err := encryptSignMessage(crypto.NewPlainMessageFromString(passphrase), addrKR, nodeKR)
	if err != nil {
		return "", "", "", err
	}

	return key, passphraseEnc, passphraseSig, nil
}
