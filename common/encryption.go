package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

type Crypto struct {
	aesGCM cipher.AEAD
}

func NewCrypto(secret []byte) (*Crypto, error) {
	c := new(Crypto)

	hkdfReader := hkdf.New(sha256.New, secret, nil, []byte("X25519-AES-GCM-Key"))
	aesKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, aesKey); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}

	c.aesGCM, err = cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Crypto) Encrypt(data []byte) ([]byte, error) {
	nonce := make([]byte, c.aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return c.aesGCM.Seal(nonce, nonce, data, nil), nil
}

func (c *Crypto) Decrypt(data []byte) ([]byte, error) {
	nonceSize := c.aesGCM.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext is too short")
	}

	nonce, d := data[:nonceSize], data[nonceSize:]

	return c.aesGCM.Open(nil, nonce, d, nil)
}
