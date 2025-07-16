package sss

import (
	"github.com/hashicorp/vault/shamir"
)

type SecretSharer interface {
	Split(secret []byte, parts, threshold int) ([][]byte, error)
	Combine(parts [][]byte) ([]byte, error)
}

type Share struct{}

func (Share) Split(secret []byte, parts, threshold int) ([][]byte, error) {
	return shamir.Split(secret, parts, threshold)
}

// Combine combines parts of a secret into a single secret
func (Share) Combine(parts [][]byte) ([]byte, error) {
	return shamir.Combine(parts)
}
