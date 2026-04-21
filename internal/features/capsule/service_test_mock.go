package capsule

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/engr-sjb/diogel/internal/features/ports"
	"github.com/engr-sjb/diogel/internal/message"
	"github.com/engr-sjb/diogel/internal/shared/database"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// mockDBStore
type mockDBStore struct {
	mock.Mock
}

func (m *mockDBStore) createOrUpdate(col database.Collection, key string, v any) error {
	args := m.Called(col, key, v)
	return args.Error(0)
}

func (m *mockDBStore) find(col database.Collection, key string, value any) (bool, error) {
	args := m.Called(col, key, value)
	return args.Bool(0), args.Error(1)
}

func (m *mockDBStore) delete(col database.Collection, key string) error {
	args := m.Called(col, key)
	return args.Error(0)
}

// mockFileStore
type mockFileStore struct {
	mock.Mock
}

func (m *mockFileStore) Open(kind ObjectStoreKind, paths []string, filesHolder []ports.File) error {
	args := m.Called(kind, paths, filesHolder)
	return args.Error(0)
}

func (m *mockFileStore) SaveCAS(hash [32]byte, data []byte) error {
	args := m.Called(hash, data)
	return args.Error(0)
}

func (m *mockFileStore) GetCAS(hash [32]byte) ([]byte, error) {
	args := m.Called(hash)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockFileStore) VerifyCAS(hash [32]byte) (bool, error) {
	args := m.Called(hash)
	return args.Bool(0), args.Error(1)
}

func (m *mockFileStore) Create(pathName string) (ports.File, error) {
	args := m.Called(pathName)
	return args.Get(0).(ports.File), args.Error(1)
}

func (m *mockFileStore) MkdirAll(dirPath string) error {
	args := m.Called(dirPath)
	return args.Error(0)
}

// Add this new mock for file capture // todo
type mockFileStoreCapture struct {
	files map[string][]byte
}

func (m *mockFileStoreCapture) MkdirAll(dirPath string) error {
	return nil
}

func (m *mockFileStoreCapture) Create(pathName string) (ports.File, error) {
	return &captureFileForValidation{
		name:  pathName,
		store: m,
	}, nil
}

// New capture file that works with the validation mock
type captureFileForValidation struct {
	name  string
	store *mockFileStoreCapture
	buf   bytes.Buffer
}

func (c *captureFileForValidation) Write(p []byte) (n int, err error) {
	return c.buf.Write(p)
}

func (c *captureFileForValidation) Close() error {
	c.store.files[c.name] = c.buf.Bytes()
	return nil
}

func (c *captureFileForValidation) Read(p []byte) (n int, err error)             { return 0, nil }
func (c *captureFileForValidation) Seek(offset int64, whence int) (int64, error) { return 0, nil }
func (c *captureFileForValidation) Stat() (os.FileInfo, error)                   { return nil, nil }

// mockRemotePeer
type mockRemotePeer struct {
	mock.Mock
}

func (m *mockRemotePeer) Write(p []byte) (int, error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}
func (m *mockRemotePeer) Read(p []byte) (int, error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}
func (m *mockRemotePeer) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockRemotePeer) Send(msg message.Msg, data []byte) (int, error) {
	args := m.Called(msg, data)
	fn, isValid := args.Get(0).(func(message.Msg, []byte) (int, error))
	if !isValid {
		return args.Int(0), args.Error(1)
	}

	return fn(msg, data)
}

func (m *mockRemotePeer) Receive(msg message.Msg, data []byte) (int, error) {
	args := m.Called(msg, data)
	fn, isValid := args.Get(0).(func(message.Msg, []byte) (int, error))
	if !isValid {
		return args.Int(0), args.Error(1)
	}

	return fn(msg, data)
}

func (m *mockRemotePeer) ID() uuid.UUID {
	args := m.Called()
	return args.Get(0).(uuid.UUID)
}

func (m *mockRemotePeer) Addr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

func (m *mockRemotePeer) PublicKey() customcrypto.PublicKeyBytes {
	args := m.Called()
	return args.Get(0).([]byte)
}

func (m *mockRemotePeer) PublicKeyStr() customcrypto.PublicKeyStr {
	args := m.Called()
	return customcrypto.PublicKeyStr(args.String(0))
}

// mockLetter implements ports.File
type mockLetter struct {
	data   []byte
	offset int64
}

func (m *mockLetter) Write(p []byte) (n int, err error) {
	m.data = append(m.data, p...)
	return len(p), nil
}

func (m *mockLetter) Read(p []byte) (n int, err error) {
	if m.data == nil {
		m.data = []byte("Dear family, message...")
	}

	// Check if we've read everything
	if m.offset >= int64(len(m.data)) {
		return 0, io.EOF
	}

	// Read from current offset
	n = copy(p, m.data[m.offset:])
	m.offset += int64(n)

	// Return EOF if we've read everything
	if m.offset >= int64(len(m.data)) {
		return n, io.EOF
	}
	return n, nil
}

func (m *mockLetter) Close() error {
	return nil
}

func (m *mockLetter) Seek(offset int64, whence int) (int64, error) {
	if m.data == nil {
		m.data = []byte("Dear family, this is my final message...")
	}

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = m.offset + offset
	case io.SeekEnd:
		newOffset = int64(len(m.data)) + offset
	default:
		return 0, errors.New("invalid whence")
	}

	if newOffset < 0 {
		return 0, errors.New("negative position")
	}

	m.offset = newOffset
	return newOffset, nil
}

func (m *mockLetter) Stat() (os.FileInfo, error) {
	if m.data == nil {
		m.data = []byte("Dear family, this is my final message...")
	}
	return &mockFileInfo{name: "letter.txt", size: int64(len(m.data))}, nil
}

type mockFileInfo struct {
	name string
	size int64
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() any           { return nil }
