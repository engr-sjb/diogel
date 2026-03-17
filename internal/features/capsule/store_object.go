/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package capsule

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/engr-sjb/diogel/internal/features/ports"
)

const (
	objectDirName = "objects"
)

type ObjectStoreKind uint8

const (
	localDisk   ObjectStoreKind = iota
	googleCloud                 // example
)

var (
	ErrFileStoreKind = errors.New("file store kind not supported")
)

type fileStorer interface {
	ports.FileStorer

	// Open takes st (storageType eg. google cloud, local disk, etc. which are
	// available as exported variables from this file), paths of the files to be
	//  opened and filesHolder, an empty slice of type port.File to be populated
	// with the opened files.
	Open(ost ObjectStoreKind, paths []string, filesHolder []ports.File) error // todo: rename this to open multiple or some.

	SaveCAS(hash [32]byte, data []byte) error
	GetCAS(hash [32]byte) ([]byte, error)
	VerifyCAS(hash [32]byte) (bool, error)
}

var _ fileStorer = (*fileStore)(nil) // To catch methods mismatches.

type FileStoreConfig struct {
	// NOTICE IMPORTANT: When you add a field, ALWAYS check if it is it's default value in its contractor func.

	// underline thing used to store object
	// s3, localDisk ,oss, cloudStorage, etc

	RootDir string
}

type fileStore struct {
	*FileStoreConfig
	// todo: if its system disk, we don't need to inject nothing, but if its some external cloud or such storage, then we will allow to inject that here.
}

func NewFileStore(cfg *FileStoreConfig) *fileStore {
	// NOTICE IMPORTANT: check if all fields on cfg are not their default value before use.

	return &fileStore{
		FileStoreConfig: cfg,
	}
}

func (s *fileStore) Save() {

}

func (s *fileStore) Open(kind ObjectStoreKind, paths []string, filesHolder []ports.File) error {
	// openedFiles := make([]ports.File, 0, len(paths))

	switch kind {
	case localDisk:
		for i := range paths {
			f, err := os.Open(paths[i])
			if err != nil {
				return err
			}
			filesHolder[i] = f
		}

	case googleCloud:
		log.Print("google cloud is unavailable")

	default:
		return ErrFileStoreKind
	}

	return nil
}

// SaveCAS stores data in content-addressable manner using hash as key
func (s *fileStore) SaveCAS(hash [32]byte, data []byte) error {
	// Todo: do proper validation for hash
	if len(hash) < 2 {
		return errors.New("hash too short for CAS storage")
	}

	// Create path: rootDir/objects/ab/cd1234...
	pathKey := CASPathTransformFunc(hash)

	// Ensure directory exists
	if err := os.MkdirAll(pathKey.dirPath, 0700); err != nil {
		return err
	}

	return os.WriteFile(pathKey.filename, data, 0600)
}

// GetCAS retrieves data by its content hash
func (s *fileStore) GetCAS(hash [32]byte) ([]byte, error) {
	if len(hash) < 2 {
		return nil, errors.New("hash too short for CAS storage")
	}

	pathKey := CASPathTransformFunc(hash)

	filePath := filepath.Join(
		s.RootDir,
		objectDirName,
		pathKey.dirPath,
		pathKey.filename,
	)
	return os.ReadFile(filePath)
}

// ExistsCAS checks if content exists in CAS
func (s *fileStore) ExistsCAS(hash [32]byte) (bool, error) {
	if len(hash) < 2 {
		return false, errors.New("hash too short for CAS storage")
	}
	pathKey := CASPathTransformFunc(hash)

	filePath := filepath.Join(
		s.RootDir,
		objectDirName,
		pathKey.dirPath,
		pathKey.filename,
	)
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// VerifyCAS retrieves data and verifies its integrity
func (s *fileStore) VerifyCAS(hash [32]byte) (bool, error) {
	// Get data
	data, err := s.GetCAS(hash)
	if err != nil {
		return false, err
	}

	// Compute hash of retrieved data
	computedHash := sha256.Sum256(data)

	// Compare with expected hash
	if computedHash != hash {
		return false, errors.New("hash verification failed: data corrupted or not available")
	}

	return true, nil
}

type pathKey struct {
	dirPath  string
	filename string
}

// CASPathTransformFunc converts a hash into an 8-level directory structure
func CASPathTransformFunc(hash [32]byte) pathKey {
	hashStr := hex.EncodeToString(hash[:])

	blockSize := 5
	sliceLen := len(hashStr) / blockSize // 64/5 = 12 blocks (with 4 chars remaining)
	paths := make([]string, sliceLen)

	for i := 0; i < sliceLen; i++ {
		from, to := i*blockSize, (i*blockSize)+blockSize
		paths[i] = hashStr[from:to]
	}

	return pathKey{
		dirPath:  filepath.Join(paths...),
		filename: hashStr[sliceLen*blockSize:], // remaining chars as filename
	}
}

func (s *fileStore) Create(pathName string) (ports.File, error) {
	fullPath := filepath.Join(s.RootDir, pathName)
	return os.Create(fullPath)
}

func (s *fileStore) MkdirAll(dirPath string) error {
	fullPath := filepath.Join(s.RootDir, dirPath)
	return os.MkdirAll(fullPath, 0700)
}
