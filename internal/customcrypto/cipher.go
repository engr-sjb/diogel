/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package customcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"log"
)

type Cipher interface {
	// Encrypt encrypts data using the provided masterKey.masterKey must be 16,
	// 24, or 32 bytes for AES-128, AES-192, or AES-256. If nonce is nil or
	// empty, a cryptographically secure random nonce is generated.Returns the
	// encrypted cipherData, the nonce used, and any error encountered.
	//
	// NOTICE IMPORTANT: Never reuse the same nonce with the same key - this breaks GCM security.
	Encrypt(masterKey, nonce, data []byte) (cipherData, usedNonce []byte, err error)
	Decrypt(masterKey []byte, nonce []byte, encData []byte) ([]byte, error)
}

// Encrypt encrypts data using AES-256-GCM with the provided masterKey.
// masterKey must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256.
// If nonce is nil or empty, a cryptographically secure random nonce is generated.
// Returns the encrypted cipherData, the nonce used, and any error encountered.
//
// NOTICE IMPORTANT: Never reuse the same nonce with the same key - this breaks GCM security.
func (c cCipher) Encrypt(masterKey, nonce, data []byte) (
	cipherData, usedNonce []byte, err error,
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
	if len(nonce) == 0 {
		nonce = make([]byte, gcm.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return cipherData, nonce, err
		}
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
