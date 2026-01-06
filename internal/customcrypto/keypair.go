/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package customcrypto

import (
	"crypto/ed25519"
	"crypto/rand"
)

type PublicKeyBytes []byte
type PrivateKeyBytes []byte
type PublicKeyStr string

func generateKeyPair() (priv []byte, pub []byte, err error) {
	pub, priv, err = ed25519.GenerateKey(rand.Reader)
	return priv, pub, err
}
