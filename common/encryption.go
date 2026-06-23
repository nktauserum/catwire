package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"io"

	"golang.org/x/crypto/hkdf"
)

func makeNonce(counter uint64) [12]byte {
	var nonce [12]byte
	binary.BigEndian.PutUint64(nonce[4:], counter)
	return nonce
}

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

func (c *Crypto) Encrypt(data []byte, counter uint64) ([]byte, error) {
	nonce := makeNonce(counter)
	return c.aesGCM.Seal(nil, nonce[:], data, nil), nil
}

func (c *Crypto) Decrypt(data []byte, counter uint64) ([]byte, error) {
	nonce := makeNonce(counter)
	return c.aesGCM.Open(nil, nonce[:], data, nil)
}
