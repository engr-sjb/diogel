package capsule

import (
	"context"
	"sync"
	"testing"

	"github.com/engr-sjb/diogel/internal/archive"
	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/engr-sjb/diogel/internal/dataredundancy"
	"github.com/engr-sjb/diogel/internal/message"
	"github.com/engr-sjb/diogel/internal/serialize"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// testHelper provides common test utilities and reduces boilerplate in tests.
// This follows the "test fixture" pattern to ensure consistent test setup.
type testHelper struct {
	t   *testing.T
	ctx context.Context
	wg  *sync.WaitGroup
}

// NewTestHelper creates a new test helper with standard context and wait group.
// Every test should use this to ensure consistent setup and proper cleanup.
func NewTestHelper(t *testing.T) *testHelper {
	return &testHelper{
		t:   t,
		ctx: context.Background(),
		wg:  &sync.WaitGroup{},
	}
}

// ServiceOption is a functional option pattern for configuring test services.
// This allows tests to override specific dependencies while keeping defaults for others.
// Example: WithMockDB(mockDB) replaces only the database, keeping real crypto.
type ServiceOption func(*ServiceConfig)

// WithMockDB injects a mock database store into the service.
// Use this when you want to verify database interactions without real I/O.
func WithMockDB(mockDB *mockDBStore) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.DBStore = mockDB
	}
}

// WithMockFileStore injects a mock file store into the service.
// Use this when you want to verify file operations without touching disk.
func WithMockFileStore(mockFS *mockFileStore) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.FileStore = mockFS
	}
}

// WithTestHooks injects test hooks to capture internal state during execution.
// This is critical for testing because it allows us to verify intermediate steps
// (like master key generation) that wouldn't normally be exposed.
func WithTestHooks(hooks *TestHooks) ServiceOption {
	return func(cfg *ServiceConfig) {
		cfg.TestHooks = hooks
	}
}

// CreateTestService creates a service with sensible defaults for testing.
// By default, it uses REAL implementations of crypto, serialization, and archiving
// because these are pure functions with no side effects - testing them together
// gives us confidence in the integration.
//
// We mock only the I/O boundaries (DB, FileStore) to avoid disk/network operations.
func (h *testHelper) CreateTestService(opts ...ServiceOption) *service {
	cfg := &ServiceConfig{
		Ctx:      h.ctx,
		Shutdown: h.wg,
		Defaults: &Defaults{
			MinNumOfGuardians: 3,
			MaxNumOfGuardians: 10,
		},
		// Use real crypto - we want to test actual encryption/decryption
		PrivateKey: make([]byte, 32),
		PublicKey:  make([]byte, 32),
		CCrypto:    customcrypto.NewCCrypto(),
		Serialize:  serialize.New(),
		Archive:    archive.NewArchive(),
		// Use real erasure coding - we want to test actual shard reconstruction
		NewErasureCoderFunc: dataredundancy.NewReedSolomonCoder,
	}

	// Apply custom options (like mocks) after defaults
	for _, opt := range opts {
		opt(cfg)
	}

	return NewService(cfg)
}

// GuardianInMemStorage tracks everything a guardian receives during capsule creation.
// This is essential for round-trip testing: we need to capture what each guardian
// gets so we can later simulate the recovery process.
//
// In production, this data would be stored in the guardian's database and file system.
// In tests, we capture it in memory for easy verification.
type GuardianInMemStorage struct {
	InitialMsg *message.CapsuleIncomingStream         // The first message with metadata
	Shards     []message.CapsuleIncomingShardStream   // All shard metadata
	ShardData  [][]byte                               // Actual shard bytes (encrypted)
	Manifest   *message.CapsuleIncomingManifestStream // The final manifest
	KeyShare   []byte                                 // The Shamir secret share
}

// CreateMockGuardians creates N mock guardian peers with unique IDs.
// Returns both the mock peers and their IDs for easy reference in tests.
//
// Why separate IDs? Because we often need to verify that messages contain
// the correct guardian IDs without calling the mock repeatedly.
func (h *testHelper) CreateMockGuardians(count int) ([]*mockRemotePeer, []uuid.UUID) {
	peers := make([]*mockRemotePeer, count)
	ids := make([]uuid.UUID, count)

	for i := range count {
		peers[i] = new(mockRemotePeer)
		ids[i] = uuid.New()
		// Setup ID() to return consistent value - this is called multiple times
		peers[i].On("ID").Return(ids[i])
	}

	return peers, ids
}

// SetupGuardianCapture configures a mock peer to capture all messages it receives.
// This is the heart of our testing strategy: instead of verifying mock calls,
// we capture the actual data and verify its correctness.
//
// Why this approach?
// 1. More realistic - we test what guardians actually receive
// 2. Easier to debug - we can inspect captured data
// 3. Enables round-trip testing - we can use captured data for reconstruction
func (h *testHelper) SetupGuardianCapture(peer *mockRemotePeer, storage *GuardianInMemStorage) {
	peer.On("Send", mock.Anything, mock.Anything).Return(
		func(msg message.Msg, data []byte) (int, error) {
			// Use type switch to handle different message types
			// Each message type represents a phase of capsule distribution
			switch m := msg.(type) {
			case *message.CapsuleIncomingStream:
				// Phase 1: Initial metadata (capsule ID, guardians, grace period)
				storage.InitialMsg = m

			case *message.CapsuleIncomingShardStream:
				// Phase 2: Encrypted data shards (the actual capsule content)
				// We make a copy because the message might be reused
				shardCopy := *m
				storage.Shards = append(storage.Shards, shardCopy)

				// Capture the actual shard bytes (encrypted data)
				if len(data) > 0 {
					// CRITICAL: Make a copy! The data slice might be reused
					storage.ShardData = append(storage.ShardData, append([]byte(nil), data...))
				}

			case *message.CapsuleIncomingManifestStream:
				// Phase 3: Manifest (map of repair groups for reconstruction)
				storage.Manifest = m

			case *message.CapsuleMasterKeyShare:
				// Phase 4: Shamir secret share (for master key reconstruction)
				// The actual share is in the data parameter, not the message
				if len(data) > 0 {
					storage.KeyShare = append([]byte(nil), data...)
				}
			}

			// Return the number of bytes "sent" (for the mock to verify)
			if data != nil {
				// CRITICAL: Return the actual data length, not the message size
				return len(data), nil
			}
			return 0, nil
		},
	)
}

// SetupMockDBAndFileStore creates standard mocks for DB and FileStore.
// These are used in almost every test, so we provide a convenience method.
//
// Why mock these?
// - DB: We don't want to spin up a real database for unit tests
// - FileStore: We don't want to write to disk during tests
//
// We use permissive mocks (accept any call) because we're testing business logic,
// not storage implementation. Storage has its own tests.
func (h *testHelper) SetupMockDBAndFileStore() (*mockDBStore, *mockFileStore) {
	mockDB := new(mockDBStore)
	mockFS := new(mockFileStore)

	// Accept any createOrUpdate call - we trust the DB implementation
	mockDB.On("createOrUpdate", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Accept any SaveCAS call - we trust the FileStore implementation
	mockFS.On("SaveCAS", mock.Anything, mock.Anything).Return(nil)

	// Accept any MkdirAll call - needed for unarchiving
	mockFS.On("MkdirAll", mock.Anything).Return(nil)

	// Accept any Create call - needed for unarchiving
	mockFS.On("Create", mock.Anything).Return(&mockLetter{}, nil)

	return mockDB, mockFS
}
