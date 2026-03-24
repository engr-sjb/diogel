/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package customcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"log"
	"sync"
)

const (
// nonceSizeStream = aead
)

type Cipher interface {
	// Encrypt encrypts data using the provided masterKey.masterKey must be 16,
	// 24, or 32 bytes for AES-128, AES-192, or AES-256. If nonce is nil or
	// empty, a cryptographically secure random nonce is generated.Returns the
	// encrypted cipherData, the nonce used, and any error encountered.
	//
	// NOTICE IMPORTANT: Never reuse the same nonce with the same key - this breaks GCM security.
	Encrypt(masterKey, nonce, data []byte) (cipherData, usedNonce []byte, err error)
	Decrypt(masterKey []byte, nonce []byte, encData []byte) ([]byte, error)
	// CopyStreamEncrypt(key []byte, src io.Reader, dst io.Writer) error
	// CopyStreamDecrypt(key []byte, src io.Reader, dst io.Writer) (int64, error)
}

type cCipher struct {
	shutdownWG *sync.WaitGroup
}

// Encrypt encrypts data using AES-256-GCM with the provided masterKey.
// masterKey must be 16, 24, or 32 bytes for AES-128, AES-192, or AES-256.
// If nonce is nil or empty, a cryptographically secure random nonce is generated.
// Returns the encrypted cipherData, the nonce used, and any error encountered.
//
// NOTICE IMPORTANT: Never reuse the same nonce with the same key - this breaks GCM security.
func (c cCipher) Encrypt(masterKey, nonce, data []byte) (
	cipherData, usedNonce []byte, err error,
) {
	// Encrypt data with master key
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return cipherData, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return cipherData, nil, err
	}

	//gen nonce
	if len(nonce) == 0 {
		nonce = make([]byte, gcm.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			return cipherData, nonce, err
		}
	}

	if len(nonce) != gcm.NonceSize() {
		return nil, nil, errors.New("provided nonce size is not equal to gcm NonceSize")
	}

	cipherData = gcm.Seal(nil, nonce[:gcm.NonceSize()], data, nil)

	return cipherData, nonce, nil
}

func (c cCipher) Decrypt(masterKey []byte, nonce []byte, encData []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, encData, nil)
}

/*
func (c cCipher) CopyStreamEncrypt(key []byte, src io.Reader, dst io.Writer) error {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return err
	}

	baseNonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, baseNonce); err != nil {
		return err
	}

	/*
		-create a bytes payload to hold all data
		- write base nonce to the payload buf
		- then write unique nonce to the payload
		- then write

		- add main nonce first to in mem buf
		- add then add chunk nonce to in mem buf
		- add
		-basically re-frame the src into a buf


	var tempBuf bytes.Buffer
	// Write base nonce to temp buf
	if _, err := tempBuf.Write(baseNonce); err != nil {
		return err
	}
	// Write base nonce
	if _, err := dst.Write(baseNonce); err != nil {
		return err
	}

	// buf := make([]byte, 32*1024)

	// chunkBufSize := 4 + aead.Overhead() + len(buf)
	// chunkBufSize := (32 * 1024) + aead.Overhead()
	chunkBufSize := (32 * 1024) + aead.NonceSize()
	chunkBuf := make([]byte, chunkBufSize)
	var payloadBuf bytes.Buffer

	payloadBuf.Write(baseNonce)

	var written int64
	var counter uint64

	for {
		n, err := src.Read(chunkBuf)
		if n > 0 {
			// Create unique nonce for this chunk
			nonce := make([]byte, aead.NonceSize())
			copy(nonce, baseNonce)
			binary.LittleEndian.PutUint64(nonce[len(nonce)-8:], counter)

			// Encrypt chunk
			cipherText := aead.Seal(
				nil,
				nonce,
				buf[:n],
				nil,
			)

			// Write chunk size then chunk
			chunkSize := make([]byte, 4)
			binary.LittleEndian.PutUint32(chunkSize, uint32(len(cipherText)))

			if _, wErr := dst.Write(chunkSize); wErr != nil {
				return written, wErr
			}
			if _, wErr := dst.Write(cipherText); wErr != nil {
				return written, wErr
			}

			written += int64(4 + len(cipherText))
			counter++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}
	}

	return nil
	// return written + int64(len(baseNonce)), nil
}

func (c cCipher) CopyStreamDecrypt(key []byte, src io.Reader, dst io.Writer) (int64, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return 0, err
	}

	// Read base nonce
	baseNonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(src, baseNonce); err != nil {
		return 0, err
	}

	var written int64
	var counter uint64
	chunkSizeBuf := make([]byte, 4)

	for {
		// Read chunk size
		_, err := io.ReadFull(src, chunkSizeBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}

		chunkSize := binary.LittleEndian.Uint32(chunkSizeBuf)
		cipherText := make([]byte, chunkSize)

		if _, err := io.ReadFull(src, cipherText); err != nil {
			return written, err
		}

		// Reconstruct nonce
		nonce := make([]byte, aead.NonceSize())
		copy(nonce, baseNonce)
		binary.LittleEndian.PutUint64(nonce[len(nonce)-8:], counter)

		// Decrypt chunk
		plaintext, err := aead.Open(nil, nonce, cipherText, nil)
		if err != nil {
			return written, err
		}

		nw, wErr := dst.Write(plaintext)
		written += int64(nw)
		if wErr != nil {
			return written, wErr
		}

		counter++
	}

	return written, nil
}

/*
func (c cCipher) CopyStreamEncrypt(key []byte, src io.Reader, dst io.Writer) (int64, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return 0, err
	}

	baseNonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, baseNonce); err != nil {
		return 0, err
	}

	// Write base nonce
	if _, err := dst.Write(baseNonce); err != nil {
		return 0, err
	}

	// buf := make([]byte, 32*1024)

	// chunkBufSize := 4 + aead.Overhead() + len(buf)
	// chunkBufSize := (32 * 1024) + aead.Overhead()
	chunkBufSize := (32 * 1024) + aead.NonceSize()
	chunkBuf := make([]byte, chunkBufSize)
	var payloadBuf bytes.Buffer

	payloadBuf.Write(baseNonce)

	var written int64
	var counter uint64



	for {
		n, err := src.Read(chunkBuf)
		if n > 0 {
			// Create unique nonce for this chunk
			nonce := make([]byte, aead.NonceSize())
			copy(nonce, baseNonce)
			binary.LittleEndian.PutUint64(nonce[len(nonce)-8:], counter)

			// Encrypt chunk
			cipherText := aead.Seal(nil, nonce, buf[:n], nil)

			// Write chunk size then chunk
			chunkSize := make([]byte, 4)
			binary.LittleEndian.PutUint32(chunkSize, uint32(len(cipherText)))

			if _, wErr := dst.Write(chunkSize); wErr != nil {
				return written, wErr
			}
			if _, wErr := dst.Write(cipherText); wErr != nil {
				return written, wErr
			}

			written += int64(4 + len(cipherText))
			counter++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}
	}

	return written + int64(len(baseNonce)), nil
}

func (c cCipher) CopyStreamDecrypt(key []byte, src io.Reader, dst io.Writer) (int64, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return 0, err
	}

	// Read base nonce
	baseNonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(src, baseNonce); err != nil {
		return 0, err
	}

	var written int64
	var counter uint64
	chunkSizeBuf := make([]byte, 4)

	for {
		// Read chunk size
		_, err := io.ReadFull(src, chunkSizeBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}

		chunkSize := binary.LittleEndian.Uint32(chunkSizeBuf)
		cipherText := make([]byte, chunkSize)

		if _, err := io.ReadFull(src, cipherText); err != nil {
			return written, err
		}

		// Reconstruct nonce
		nonce := make([]byte, aead.NonceSize())
		copy(nonce, baseNonce)
		binary.LittleEndian.PutUint64(nonce[len(nonce)-8:], counter)

		// Decrypt chunk
		plaintext, err := aead.Open(nil, nonce, cipherText, nil)
		if err != nil {
			return written, err
		}

		nw, wErr := dst.Write(plaintext)
		written += int64(nw)
		if wErr != nil {
			return written, wErr
		}

		counter++
	}

	return written, nil
}


*/
