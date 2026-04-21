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

	//Todo: We need to add a header to each shard that has its index and its repairID in it.
	for i, shard := range shards {
		//Todo: create header and shard in-place.
		shards[i] = append([]byte{uint8(i)}, shard...)
	}

	return shards, nil
}

// Reconstruct rebuilds original data from available shards
func (self *erasureCode) Reconstruct(shards [][]byte, dst io.Writer) error {
	// Todo: Might have to reconsider if we returning shards or we take a dst. which means we need a writer.

	//Todo: reindex shards so we know we have them in the right index

	// Extract index and rebuild proper array
	totalShards := self.dataShardNum + self.parityShardNum
	if len(shards) != totalShards {
		return fmt.Errorf(
			"incorrect number of shards: expected '%d', but got '%d'/Tip: Make sure missing shards are nil but the length off the shards need to be %d /",
			totalShards,
			len(shards),
			totalShards,
		)
	}

	availableCount := 0
	indexedShards := make([][]byte, totalShards)
	for _, shard := range shards {
		if len(shard) == 0 || shard == nil {
			continue
		}

		indexByte := shard[0]
		index := int(indexByte) // Explicit conversion

		// Validate index is within bounds
		if index < 0 || index >= totalShards {
			return fmt.Errorf(
				"shard index out of bounds: got %d, expected 0-%d",
				index,
				totalShards-1,
			)
		}

		shardData := shard[1:]
		indexedShards[index] = shardData
		availableCount++
	}

	if availableCount < self.dataShardNum {
		return fmt.Errorf(
			"insufficient shards for reconstruction: have %d, need %d",
			availableCount,
			self.dataShardNum,
		)
	}

	err := self.encoder.Reconstruct(indexedShards)
	if err != nil {
		return fmt.Errorf(
			"failed to reconstruct shards: %w",
			err,
		)
	}

	isValid, err := self.encoder.Verify(indexedShards)
	if err != nil {
		return err
	}

	if !isValid {
		return fmt.Errorf(
			"Verification failed after reconstruction, data likely corrupted.",
		)
	}

	shardLen := len(indexedShards[0])

	var buf bytes.Buffer
	err = self.encoder.Join(&buf, indexedShards, shardLen*self.dataShardNum)
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
