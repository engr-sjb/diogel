// Package dataredundancy provides erasure coding and reconstruction.
package dataredundancy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/klauspost/reedsolomon"
)

const (
	headerLength = 8
)

// ErasureCoder handles data erasure coding and reconstruction
type ErasureCoder interface {
	Erasure(data []byte) ([][]byte, error)
	Reconstruct(shards [][]byte, dst io.Writer) error
}

// NewErasureCoderFunc returns an erasureCoder that handles data erasure coding and reconstruction
type NewErasureCoderFunc func(dataShardsNum, parityShardsNum int) (*erasureCode, error)

// ErasureFunc
type ErasureFunc func(data []byte) ([][]byte, error)

// ReconstructFunc
type ReconstructFunc func(shards [][]byte, dst io.Writer) ([]byte, error)

// erasureCode implements ErasureCoder using Reed-Solomon erasure coding
type erasureCode struct {
	encoder                      reedsolomon.Encoder
	dataShardNum, parityShardNum int
}

// NewReedSolomonCoder creates a new Reed-Solomon erasure coder
func NewReedSolomonCoder(dataShardsNum, parityShardsNum int) (*erasureCode, error) {
	encoder, err := reedsolomon.New(
		dataShardsNum,
		parityShardsNum,
	)

	if err != nil {
		return nil, err
	}
	return &erasureCode{
		encoder:        encoder,
		dataShardNum:   dataShardsNum,
		parityShardNum: parityShardsNum,
	}, nil
}

// Erasure splits data into erasure-coded shards
func (self *erasureCode) Erasure(data []byte) ([][]byte, error) {
	lengthBytes := make([]byte, headerLength)
	binary.BigEndian.PutUint64(lengthBytes, uint64(len(data)))
	dataWithLength := append(lengthBytes, data...)

	shards, err := self.encoder.Split(dataWithLength)
	if err != nil {
		return nil, err
	}

	err = self.encoder.Encode(shards)
	if err != nil {
		return nil, err
	}

	return shards, nil
}

// Reconstruct rebuilds original data from available shards
func (self *erasureCode) Reconstruct(shards [][]byte, dst io.Writer) error {
	// Todo: Might have to reconsider if we returning shards or we take a dst. which means we need a writer.
	err := self.encoder.Reconstruct(shards)
	if err != nil {
		return fmt.Errorf(
			"failed to reconstruct shards: %w",
			err,
		)
	}

	isValid, err := self.encoder.Verify(shards)
	if err != nil {
		return err
	}

	if !isValid {
		return fmt.Errorf(
			"Verification failed after reconstruction, data likely corrupted.",
		)
	}

	shardLen := len(shards[0])

	var buf bytes.Buffer
	err = self.encoder.Join(&buf, shards, shardLen*self.dataShardNum)
	if err != nil {
		return fmt.Errorf(
			"failed to join reconstructed shards: %w",
			err,
		)
	}

	reconstructed := buf.Bytes()
	originalLen := binary.BigEndian.Uint64(reconstructed[:headerLength])
	data := reconstructed[headerLength : headerLength+originalLen]

	_, err = dst.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}
