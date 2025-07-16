package customcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"log"
)

type Cipher interface {
	Encrypt(data []byte, key []byte) (cipherData, nonce []byte, err error)
	Decrypt(masterKey []byte, nonce []byte, encData []byte) ([]byte, error)
}

type cCipher struct{}

func (c cCipher) Encrypt(data []byte, masterKey []byte) (
	cipherData, nonce []byte, err error,
) {
	// Encrypt data with master key
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return cipherData, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return cipherData, nil, err
	}

	//gen nonce
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return cipherData, nonce, err
	}

	cipherData = gcm.Seal(nil, nonce, data, nil)

	return cipherData, nonce, nil
}

func (c cCipher) Decrypt(masterKey []byte, nonce []byte, encData []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, encData, nil)
}
