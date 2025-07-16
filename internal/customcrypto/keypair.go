package customcrypto

import (
	"crypto/ed25519"
	"crypto/rand"
)

func generateKeyPair() (priv []byte, pub []byte, err error) {
	pub, priv, err = ed25519.GenerateKey(rand.Reader)
	return priv, pub, err
}
