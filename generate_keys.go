package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"log"
)

func main() {
	curve := ecdh.X25519()
	clientPrivateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	clientPublicKey := clientPrivateKey.PublicKey()

	pubKeyStr := base64.StdEncoding.EncodeToString(clientPublicKey.Bytes())
	log.Println("Public Key:", pubKeyStr)

	privKeyStr := base64.StdEncoding.EncodeToString(clientPrivateKey.Bytes())
	log.Println("Private Key:", privKeyStr)

}
