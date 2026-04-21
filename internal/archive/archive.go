/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package archive

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/engr-sjb/diogel/internal/features/ports"
	"github.com/klauspost/compress/zstd"
)

var (
	ErrCreatingHeader       = errors.New("archiver: err creating header")
	ErrStreamingToDst       = errors.New("archiver: failed to streaming file to dst")
	ErrGettingFileStat      = errors.New("archiver: error getting file stat")
	ErrFailedCreatingHeader = errors.New("archiver: error creating zip header for file")
	ErrCompressingStream    = errors.New("archiver: failed to compress stream")
	ErrDecompressingStream  = errors.New("archiver: failed to decompress stream")
)

const maxNumOfWorkers uint8 = 8

var (
	numOfWorkers = func() uint8 {
		num := uint8(runtime.NumCPU())
		if num <= 4 {
			return max(1, (num / 2))
		}

		num = (num / 2) + 1

		if num > maxNumOfWorkers {
			return maxNumOfWorkers
		}

		return num
	}()
)

type Archiver interface {
	ArchiveStream(ctx context.Context, files []ports.File, dst io.WriteCloser) error
}

var _ Archiver = (*archive)(nil)

type archive struct {
	shutdownWG *sync.WaitGroup
}

func NewArchive() *archive {
	return &archive{}
}

func (self *archive) ArchiveStream(ctx context.Context, files []ports.File, dst io.WriteCloser) error {
	defer dst.Close()

	zstdW, err := zstd.NewWriter(dst)
	if err != nil {
		return fmt.Errorf("%v: %w", ErrDecompressingStream, err)
	}
	defer zstdW.Close()

	tarW := tar.NewWriter(zstdW)
	defer tarW.Close()

	for i := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		entry := files[i]
		stat, err := entry.Stat()
		if err != nil {
			return fmt.Errorf(
				"%v: %w",
				ErrGettingFileStat,
				err,
			)
		}
		hdr := &tar.Header{
			Name:    stat.Name(),
			Mode:    int64(stat.Mode()),
			Size:    stat.Size(),
			ModTime: stat.ModTime(),
		}

		if err := tarW.WriteHeader(hdr); err != nil {
			return fmt.Errorf("%v for file '%s': %w", ErrCreatingHeader, stat.Name(), err)
		}

		if seeker, ok := entry.(io.Seeker); ok {
			seeker.Seek(0, io.SeekStart)
		}

		if _, err := io.Copy(tarW, entry); err != nil {
			return fmt.Errorf("%v for file %s: %w", ErrStreamingToDst, stat.Name(), err)
		}
	}

	return tarW.Close()
}

func (self *archive) UnArchiveStream(ctx context.Context, src io.Reader, fileStore ports.FileStorer) error {
	zstdR, err := zstd.NewReader(src)
	if err != nil {
		return fmt.Errorf("%v: %w", ErrDecompressingStream, err)
	}
	defer zstdR.Close()

	tarR := tar.NewReader(zstdR)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		hdr, err := tarR.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			// todo: use the right error type here to return.
			return fmt.Errorf("%v: %w", ErrDecompressingStream, err)
		}

		if hdr.FileInfo().IsDir() {
			if err := fileStore.MkdirAll(hdr.Name); err != nil {
				return err
			}
			continue
		}

		if dir := filepath.Dir(hdr.Name); dir != "." && dir != "/" {
			if err := fileStore.MkdirAll(dir); err != nil {
				return err
			}
		}

		fDst, err := fileStore.Create(hdr.Name)
		if err != nil {
			return fmt.Errorf("%v for file '%s': %w", ErrDecompressingStream, hdr.Name, err)
		}

		if _, err := io.Copy(fDst, tarR); err != nil {
			fDst.Close()
			return fmt.Errorf("%v for file '%s': %w", ErrStreamingToDst, hdr.Name, err)
		}
		fDst.Close()
	}

	return nil
}

/*
func (c *archive) Decompress(ctx context.Context, src io.Reader, dst io.Writer) error {
	return nil
}


c.shutdownWG.Go(func() {
		zipW := zip.NewWriter(dst)
		defer c.shutdownWG.Done()
		defer dst.Close()
		defer zipW.Close()

		for i := range files {
			entry := files[i]
			stat, err := entry.Stat()
			if err != nil {
				log.Printf(
					"err in the file zipping getting stat: %v",
					err,
				) //todo : we need to find a way to return this to the user.
				break
			}

			fh := &zip.FileHeader{
				Name:   stat.Name(),
				Method: zip.Deflate,
			}

			if !stat.ModTime().IsZero() {
				fh.Modified = stat.ModTime()
			}

			if stat.Mode() != 0 {
				fh.SetMode(stat.Mode())
			}

			dst, err := zipW.CreateHeader(fh)
			if err != nil {
				log.Printf(
					"err in the file zipping creating header for %s: %v",
					stat.Name(),
					err,
				) //todo : we need to find a way to return this to the user.
				break
			}

			if seeker, ok := entry.(io.Seeker); ok {
				seeker.Seek(0, io.SeekStart)
			}

			if _, err := io.Copy(dst, entry); err != nil {
				log.Printf(
					"err streaming file to dst %s: %v",
					stat.Name(),
					err,
				) //todo : we need to find a way to return this to the user.

				zipW.Close()
				break
			}

		}

		if err := zipW.Close(); err != nil {
			log.Printf(
				"err in the file zipping files:  %v",
				err,
			) //todo : we need to find a way to return this to the user.
		}
	})
*/
