/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package archive

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/engr-sjb/diogel/internal/features/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFileStore struct {
	files map[string]*bytes.Buffer
	dirs  map[string]bool
}

func newMockFileStore() *mockFileStore {
	return &mockFileStore{
		files: make(map[string]*bytes.Buffer),
		dirs:  make(map[string]bool),
	}
}

func (m *mockFileStore) Create(path string) (ports.File, error) {
	buf := &bytes.Buffer{}
	m.files[path] = buf
	return &mockFile{
		name:   path,
		buffer: buf,
	}, nil
}

func (m *mockFileStore) MkdirAll(path string) error {
	m.dirs[path] = true
	return nil
}

type mockFile struct {
	name   string
	buffer *bytes.Buffer
}

func (m *mockFile) Read(p []byte) (n int, err error) {
	return m.buffer.Read(p)
}

func (m *mockFile) Write(p []byte) (n int, err error) {
	return m.buffer.Write(p)
}

func (m *mockFile) Close() error {
	return nil
}

func (m *mockFile) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (m *mockFile) Stat() (os.FileInfo, error) {
	return &mockFileInfo{name: m.name}, nil
}

type mockFileInfo struct {
	name string
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() any           { return nil }

func TestArchiveStream(t *testing.T) {
	tests := []struct {
		name        string
		files       []ports.File
		expectError bool
		errorMsg    string
	}{
		{
			name: "single file",
			files: []ports.File{
				&ports.FileMem{
					Name:    "test.txt",
					Content: io.NopCloser(strings.NewReader("hello world")),
					Mode:    0644,
					ModTime: time.Now(),
					Size:    11,
				},
			},
			expectError: false,
		},
		{
			name: "multiple files",
			files: []ports.File{
				&ports.FileMem{
					Name:    "file1.txt",
					Content: io.NopCloser(strings.NewReader("content 1")),
					Mode:    0644,
					ModTime: time.Now(),
					Size:    9,
				},
				&ports.FileMem{
					Name:    "file2.txt",
					Content: io.NopCloser(strings.NewReader("content 2")),
					Mode:    0644,
					ModTime: time.Now(),
					Size:    9,
				},
			},
			expectError: false,
		},
		{
			name:        "empty files slice",
			files:       []ports.File{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiver := NewArchive()
			ctx := context.Background()
			dst := &bytes.Buffer{}

			err := archiver.ArchiveStream(ctx, tt.files, &nopWriteCloser{dst})

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Greater(t, dst.Len(), 0, "archived data should not be empty")
			}
		})
	}
}

func TestArchiveStreamContextCancellation(t *testing.T) {
	archiver := NewArchive()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	files := []ports.File{
		&ports.FileMem{
			Name:    "test.txt",
			Content: io.NopCloser(strings.NewReader("hello")),
			Mode:    0644,
			ModTime: time.Now(),
			Size:    5,
		},
	}

	dst := &bytes.Buffer{}
	err := archiver.ArchiveStream(ctx, files, &nopWriteCloser{dst})

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestUnArchiveStream(t *testing.T) {
	tests := []struct {
		name          string
		setupArchive  func() io.Reader
		expectedFiles map[string]string
		expectError   bool
	}{
		{
			name: "single file extraction",
			setupArchive: func() io.Reader {
				archiver := NewArchive()
				files := []ports.File{
					&ports.FileMem{
						Name:    "test.txt",
						Content: io.NopCloser(strings.NewReader("hello world")),
						Mode:    0644,
						ModTime: time.Now(),
						Size:    11,
					},
				}
				dst := &bytes.Buffer{}
				err := archiver.ArchiveStream(context.Background(), files, &nopWriteCloser{dst})
				require.NoError(t, err)
				return dst
			},
			expectedFiles: map[string]string{
				"test.txt": "hello world",
			},
			expectError: false,
		},
		{
			name: "multiple files extraction",
			setupArchive: func() io.Reader {
				archiver := NewArchive()
				files := []ports.File{
					&ports.FileMem{
						Name:    "file1.txt",
						Content: io.NopCloser(strings.NewReader("content 1")),
						Mode:    0644,
						ModTime: time.Now(),
						Size:    9,
					},
					&ports.FileMem{
						Name:    "file2.txt",
						Content: io.NopCloser(strings.NewReader("content 2")),
						Mode:    0644,
						ModTime: time.Now(),
						Size:    9,
					},
				}
				dst := &bytes.Buffer{}
				err := archiver.ArchiveStream(context.Background(), files, &nopWriteCloser{dst})
				require.NoError(t, err)
				return dst
			},
			expectedFiles: map[string]string{
				"file1.txt": "content 1",
				"file2.txt": "content 2",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiver := NewArchive()
			ctx := context.Background()
			mockStore := newMockFileStore()

			src := tt.setupArchive()
			err := archiver.UnArchiveStream(ctx, src, mockStore)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expectedFiles), len(mockStore.files))

				for filename, expectedContent := range tt.expectedFiles {
					buf, exists := mockStore.files[filename]
					assert.True(t, exists, "file %s should exist", filename)
					assert.Equal(t, expectedContent, buf.String())
				}
			}
		})
	}
}

func TestArchiveUnArchiveRoundTrip(t *testing.T) {
	archiver := NewArchive()
	ctx := context.Background()

	originalFiles := map[string]string{
		"letter.txt":     "This is my final message",
		"document.txt":   "Important document content",
		"credentials.md": "Secret credentials here",
	}

	files := make([]ports.File, 0, len(originalFiles))
	for name, content := range originalFiles {
		files = append(files, &ports.FileMem{
			Name:    name,
			Content: io.NopCloser(strings.NewReader(content)),
			Mode:    0644,
			ModTime: time.Now(),
			Size:    int64(len(content)),
		})
	}

	archiveBuf := &bytes.Buffer{}
	err := archiver.ArchiveStream(ctx, files, &nopWriteCloser{archiveBuf})
	require.NoError(t, err)

	mockStore := newMockFileStore()
	err = archiver.UnArchiveStream(ctx, archiveBuf, mockStore)
	require.NoError(t, err)

	assert.Equal(t, len(originalFiles), len(mockStore.files))
	for filename, expectedContent := range originalFiles {
		buf, exists := mockStore.files[filename]
		assert.True(t, exists, "file %s should exist after extraction", filename)
		assert.Equal(t, expectedContent, buf.String())
	}
}

func TestUnArchiveStreamContextCancellation(t *testing.T) {
	archiver := NewArchive()
	ctx, cancel := context.WithCancel(context.Background())

	files := []ports.File{
		&ports.FileMem{
			Name:    "test.txt",
			Content: io.NopCloser(strings.NewReader("hello")),
			Mode:    0644,
			ModTime: time.Now(),
			Size:    5,
		},
	}

	archiveBuf := &bytes.Buffer{}
	err := archiver.ArchiveStream(context.Background(), files, &nopWriteCloser{archiveBuf})
	require.NoError(t, err)

	cancel()
	mockStore := newMockFileStore()
	err = archiver.UnArchiveStream(ctx, archiveBuf, mockStore)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestUnArchiveStreamInvalidData(t *testing.T) {
	archiver := NewArchive()
	ctx := context.Background()
	mockStore := newMockFileStore()

	invalidData := strings.NewReader("this is not a valid zstd+tar archive")
	err := archiver.UnArchiveStream(ctx, invalidData, mockStore)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decompress stream")
}

type nopWriteCloser struct {
	io.Writer
}

func (n *nopWriteCloser) Close() error {
	return nil
}
