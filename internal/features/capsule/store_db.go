/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package capsule

import (
	"encoding/json"
	"errors"
	"log"

	"github.com/engr-sjb/diogel/internal/shared/database"
	bolt "go.etcd.io/bbolt"
	boltErr "go.etcd.io/bbolt/errors"
)

var (
	ErrDataNotFound = errors.New("data not found")
)

type dbStorer interface {
	createOrUpdate(col database.Collection, key string, v any) error
	find(col database.Collection, key string, value any) (exists bool, err error)
	delete(col database.Collection, key string) error
}

type DBStoreConfig struct {
	// NOTICE IMPORTANT: When you add a field, ALWAYS check if it is it's default value in its contractor func.

	DB *bolt.DB
}

type dbStore struct {
	*DBStoreConfig
}

func NewDBStore(cfg *DBStoreConfig) *dbStore {
	// NOTICE IMPORTANT: check if all fields on cfg are not their default value before use.
	switch {
	case cfg == nil:
		log.Fatalln("store config is nil")
	case cfg.DB == nil:
		log.Fatalln("invalid store config: DB is nil")
	}

	return &dbStore{
		DBStoreConfig: cfg,
	}
}

func (s *dbStore) createOrUpdate(coll database.Collection, key string, v any) error {
	//Todo: Use a bytes format here instead of marshalling every time.
	bv, err := json.Marshal(v)
	if err != nil {
		return err
	}

	err = s.DB.Update(
		func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists(
				[]byte(coll.BucketName()),
			)
			if err != nil {
				return err
			}

			return b.Put([]byte(key), bv)
		},
	)
	if err != nil {
		return err
	}

	return nil
}

// find populates into `value` a []byte. So you are to pass the right type as a pointer value in 'value'.
func (s *dbStore) find(coll database.Collection, key string, value any) (exists bool, err error) {
	var out []byte
	err = s.DB.View(
		func(tx *bolt.Tx) error {
			b := tx.Bucket(
				[]byte(coll.BucketName()),
			)
			if b == nil {
				return boltErr.ErrBucketNotFound
				// todo: think about it. the type of errors when bucket doesn't exist
			}

			out = b.Get([]byte(key))
			if out == nil {
				return ErrDataNotFound
			}

			return json.Unmarshal(out, value)
		},
	)
	if err != nil {
		if errors.Is(err, boltErr.ErrBucketNotFound) || errors.Is(err, ErrDataNotFound) {
			return false, nil
			// todo: think about it. the type of errors when bucket doesn't exist
			//todo: might have check why i have two bucket not found above and here.
		}
		return false, err
	}

	return true, nil
}

func (s *dbStore) delete(coll database.Collection, key string) error {
	err := s.DB.Update(
		func(tx *bolt.Tx) error {
			b := tx.Bucket(
				[]byte(coll.BucketName()),
			)
			if b == nil {
				return boltErr.ErrBucketNotFound
			}

			return b.Delete([]byte(key))
		},
	)
	if err != nil {
		return err
	}

	return nil
}
