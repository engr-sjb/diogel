package user

import (
	"encoding/json"
	"errors"
	"log"

	bolt "go.etcd.io/bbolt"
	boltErr "go.etcd.io/bbolt/errors"
)

type key string

var (
	ErrDataNotFound = errors.New("data not found")
)

type dbStorer interface {
	find(key key, value any) (exists bool, err error)
	save(key key, v any) error
	delete(key key) error
}

type DBStoreConfig struct {
	// NOTICE IMPORTANT: When you add a field, ALWAYS check if it is it's default value in its contractor func.

	DB                    *bolt.DB
	UserBucketName        string
	UserSettingBucketName string
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
	case cfg.UserBucketName == "":
		log.Fatalln("invalid store config: UserBucketName is empty")
	case cfg.UserSettingBucketName == "":
		log.Fatalln("invalid store config: UserSettingBucketName is empty")
	}

	return &dbStore{
		DBStoreConfig: cfg,
	}
}

// find returns a []byte, and you are to type convert it to the type of data you
// are expecting. If you are expecting a string for the key, you do [string(value)].
func (s *dbStore) find(key key, value any) (exists bool, err error) {
	var out []byte
	err = s.DB.View(
		func(tx *bolt.Tx) error {
			b := tx.Bucket(
				[]byte(s.UserBucketName),
			)
			if b == nil {
				return boltErr.ErrBucketNotFound
				// todo: think about it. the type of errors when bucket doesn't exist
			}

			out = b.Get([]byte(key))
			if out == nil {
				return ErrDataNotFound
			}

			// todo: use out serializer package or ust use gob here. not sure yet if i will inject serializer into store directly or pass it from service.
			return json.Unmarshal(out, value)
		},
	)
	if err != nil {
		if errors.Is(err, boltErr.ErrBucketNotFound) {
			return false, nil
			// todo: think about it. the type of errors when bucket doesn't exist
		}
		if errors.Is(err, ErrDataNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (s *dbStore) save(key key, v any) error {
	bv, err := json.Marshal(v)
	if err != nil {
		log.Println(err)
		return err
	}

	err = s.DB.Update(
		func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists(
				[]byte(s.UserBucketName),
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

func (s *dbStore) delete(key key) error {
	err := s.DB.Update(
		func(tx *bolt.Tx) error {
			b := tx.Bucket(
				[]byte(s.UserBucketName),
			)

			return b.Delete([]byte(key))
		},
	)
	if err != nil {
		return err
	}

	return nil
}
