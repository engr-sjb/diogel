package storage

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

func NewBBolt(dir string, logger *slog.Logger) *bolt.DB {
	keyPath := fmt.Sprintf(
		"%s/db",
		dir,
	)

	err := os.MkdirAll(
		keyPath,
		0700,
	)
	if err != nil {
		log.Fatal("Error creating directory")
	}

	boltDB, err := bolt.Open(
		fmt.Sprintf(
			"%s/diogel.db", // todo : import cfg in here for path
			keyPath,
		),
		0600,
		&bolt.Options{
			Timeout: 1 * time.Second,
		},
	)
	if err != nil {
		log.Fatal("Error opening bbolt database")
	}

	log.Println("bbolt db connected!")

	return boltDB
}
