package user

import (
	"context"
	"log"
	"log/slog"

	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/engr-sjb/diogel/internal/peererrors"
)

const (
	identityKey key = "identity"
)

type servicer interface {
	// InitIdentity initializes the Servicer's cryptographic identity.
	// Returns an error if any cryptographic operations or database access fails.
	InitIdentity(password string) error
	// GetKeyPair returns the user's private and public key pair.
	// Returns nil for if they are not set.
	GetKeyPair() (privateKey []byte, PublicKey []byte)
}

type ServiceConfig struct {
	// NOTICE IMPORTANT: When you add a field, ALWAYS check if it is it's default value in its contractor func.

	Ctx     context.Context
	DBStore dbStorer
	CCrypto *customcrypto.CCrypto
	Logger  *slog.Logger
}

type service struct {
	*ServiceConfig
	privateKey []byte
	publicKey  []byte
}

func NewService(cfg *ServiceConfig) *service {
	// NOTICE IMPORTANT: check if all fields on cfg are not their default value before use.
	switch {
	case cfg == nil:
		log.Fatal("service config cannot be nil")
	case cfg.Ctx == nil:
		log.Fatal("ctx cannot be nil")
	case cfg.DBStore == nil:
		log.Fatal("DBStore cannot be nil")
	case cfg.CCrypto == nil:
		log.Fatal("CCrypto cannot be nil")
	case cfg.Logger == nil:
		log.Fatal("Logger cannot be nil")
	}

	s := &service{
		ServiceConfig: cfg,
	}

	return s
}

// InitIdentity initializes the user identity by either:
//   - Loading an existing identity from the database, decrypting the private key using a derived key from the password
//   - Creating a new identity by generating a key pair, encrypting the private key using a derived key from the password, and storing in database
func (s *service) InitIdentity(password string) error {
	// ask for password
	// pwd, err := promptPassword()
	// if err != nil {
	// 	return peererrors.New(
	// 		peererrors.CodeDefault,
	// 		err.Error(),
	// 	)
	// }

	exists, err := s.existingIdentity(password)
	if err != nil {
		return peererrors.New(
			peererrors.CodeDefault,
			err.Error(),
		)
	}
	if exists {
		return nil
	}

	log.Println("creating new identity")
	// create key
	derivedKey, usedSalt, err := s.CCrypto.DeriveKey(
		[]byte(password),
		nil,
	)
	if err != nil {
		return peererrors.New(
			peererrors.CodeDefault,
			err.Error(),
		)
	}

	//key pair
	newPrivKey, newPubKey, err := s.CCrypto.GenerateKeyPair()
	if err != nil {
		return peererrors.New(
			peererrors.CodeDefault,
			err.Error(),
		)
	}

	// encrypt new private
	newEncPrivKey, usedNonce, err := s.CCrypto.Cipher.Encrypt(
		newPrivKey,
		derivedKey,
	)
	if err != nil {
		return peererrors.New(
			peererrors.CodeDefault,
			err.Error(),
		)
	}

	// save new identity
	newIdentity := &identity{
		EncPrivKey: newEncPrivKey,
		PublicKey:  newPubKey,
		Salt:       usedSalt,
		Nonce:      usedNonce,
	}
	if err = s.DBStore.save(
		identityKey,
		newIdentity,
	); err != nil {
		return peererrors.New(
			peererrors.CodeDefault,
			err.Error(),
		)
	}

	// set keys
	s.privateKey = newPrivKey
	s.publicKey = newPubKey

	return nil
}

func (s *service) existingIdentity(pwd string) (bool, error) {
	retrievedIdentity := new(identity)
	exists, err := s.DBStore.find(
		identityKey,
		retrievedIdentity,
	)
	if err != nil {
		return false, peererrors.New(
			peererrors.CodeDefault,
			err.Error(),
		)
	}

	if !exists {
		return exists, nil
	}

	log.Println("loading existing identity")
	derivedKey, _, err := s.CCrypto.DeriveKey(
		[]byte(pwd),
		retrievedIdentity.Salt,
	)
	if err != nil {
		return false, peererrors.New(
			peererrors.CodeDefault,
			err.Error(),
		)
	}

	decPrivateKey, err := s.CCrypto.Cipher.Decrypt(
		derivedKey,
		retrievedIdentity.Nonce,
		retrievedIdentity.EncPrivKey,
	)
	if err != nil {
		return false, peererrors.New(
			peererrors.CodeDefault,
			err.Error(),
		)
	}

	s.privateKey = decPrivateKey
	s.publicKey = retrievedIdentity.PublicKey

	return exists, nil
}

// GetKeyPair returns the user's private and public key pair that was previously initialized
// via InitIdentity(). The private key is the decrypted version that was either loaded from
// storage or newly generated. The public key is the corresponding public key for the private key.
//
// IMPORTANT NOTE: RETURNS NIL FOR BOTH KEYS IF IDENTITY HAS NOT BEEN INITIALIZED. SO CALL InitIdentity() METHOD BEFORE CALLING GetKeyPair().
func (s *service) GetKeyPair() (privKey []byte, PubKey []byte) {
	return s.privateKey, s.publicKey
}
