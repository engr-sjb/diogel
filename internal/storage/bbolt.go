package storage

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	bolt "go.etcd.io/bbolt"
)

func NewBBolt(logger *slog.Logger) *bolt.DB {
	// home, err := os.UserHomeDir()
	// if err != nil {
	// 	log.Fatalf("Error getting user home directory: %v", err)
	// }

	// keyPath := filepath.Join(
	//home,
	// 	"./.diogel", // todo: pull name from cfg
	// )

	keyPath := "./.diogel"

	os.MkdirAll(
		keyPath,
		0700,
	)

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
