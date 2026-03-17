/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package ports

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type FileStorer interface {
	// For writing extracted files during recovery
	Create(path string) (File, error)
	MkdirAll(path string) error
}

type File interface {
	io.ReadWriteCloser
	io.Seeker
	Stat() (fs.FileInfo, error)
}

var _ File = (*FileMem)(nil)

// FileMem represents a file in memory. (A file inside in mem that can be used in a Zip file.)
type FileMem struct {
	Name    string        // Name of the file inside the Zip. (eg."dir/file.txt")
	Content io.ReadCloser // This can be a *os.File, strings.Reader, etc.
	Mode    os.FileMode
	ModTime time.Time
	Size    int64
}

func (f *FileMem) Read(p []byte) (n int, err error) {
	return f.Content.Read(p)
}

func (f *FileMem) Write(p []byte) (n int, err error) {
	if w, ok := f.Content.(io.Writer); ok {
		return w.Write(p)
	}

	return 0, nil
}

func (f *FileMem) Close() error {
	return f.Content.Close()
}

func (f *FileMem) Seek(offset int64, whence int) (int64, error) {
	if f, ok := f.Content.(io.Seeker); ok {
		return f.Seek(offset, whence)
	}

	return 0, nil
}

func (f *FileMem) Stat() (os.FileInfo, error) {
	return &fileInfo{
		FName:    f.Name,
		FMode:    f.Mode,
		FModTime: f.ModTime,
		FSize:    f.Size,
	}, nil
}

type fileInfo struct {
	FName    string
	FSize    int64
	FMode    os.FileMode
	FModTime time.Time
}

func (f *fileInfo) Name() string {
	return filepath.Base(f.FName)
}

func (f *fileInfo) ModTime() time.Time {
	return f.FModTime
}

func (f *fileInfo) Mode() fs.FileMode {
	return f.FMode
}

func (f *fileInfo) Size() int64 {
	return f.FSize
}

func (f *fileInfo) IsDir() bool {
	return f.Mode().IsDir()
}

func (f *fileInfo) Sys() any {
	return nil
}
