package customcrypto

import (
	"crypto/rand"

	"golang.org/x/crypto/scrypt"
)

// deriveKey is a Key Derive Function(KDF).
func deriveKey(password []byte, salt []byte) (derivedKey, usedSalt []byte, err error) {
	const (
		keyLen     = 32
		iterations = 32768
		r          = 8
		p          = 1
	)

	if salt != nil {
		usedSalt = salt
	} else {
		usedSalt, err = newSalt()
		if err != nil {
			return nil, nil, err
		}
	}

	derivedKey, err = scrypt.Key(password, usedSalt, iterations, r, p, keyLen)

	return
}

func newSalt() ([]byte, error) {
	salt := make([]byte, 32)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, err
	}
	return salt, nil
}
