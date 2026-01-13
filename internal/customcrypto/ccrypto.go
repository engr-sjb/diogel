/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package customcrypto

import (
	"log"

	"github.com/engr-sjb/diogel/internal/sss"
)

// CCrypto provides cryptographic functionality including encryption/decryption,
// key generation, key derivation and secret sharing capabilities
//
// Fields:
//   - Cipher: Interface for encryption and decryption operations
//   - GenerateKeyPair: Function to generate a new public/private key pair
//   - DeriveKey: Function to derive a cryptographic key from a password and salt
//   - SecretSharer: Interface for splitting secrets into shares and reconstructing them
type CCrypto struct {
	Cipher          Cipher
	GenerateKeyPair func() (priv []byte, pub []byte, err error)
	DeriveKey       func(password []byte, salt []byte) (derivedKey, usedSalt []byte, err error)
	SecretSharer    sss.SecretSharer
}

func NewCCrypto() CCrypto {
	cc := CCrypto{
		Cipher:          &cCipher{},
		GenerateKeyPair: generateKeyPair,
		DeriveKey:       deriveKey,
		SecretSharer:    &sss.Share{},
	}

	switch {
	case cc.Cipher == nil:
		log.Println("cc.Cipher is nil")
	case cc.DeriveKey == nil:
		log.Println("cc.DeriveKey is nil")
	case cc.GenerateKeyPair == nil:
		log.Println("cc.GenerateKeyPair is nil")
	case cc.SecretSharer == nil:
		log.Println("cc.SecretSharer is nil")
	}

	return cc
}
