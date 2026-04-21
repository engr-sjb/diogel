package dataredundancy

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErasure(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		dataShards   int
		parityShards int
		// corruptShards      []int
		corruptShardsCount int
	}{
		{
			name:               "reconstruct with all shards",
			data:               []byte("hello world from space"),
			dataShards:         3,
			parityShards:       2,
			corruptShardsCount: 0,
		},
		{
			name:               "reconstruct with one missing shard",
			data:               []byte("hello world from space from one missing shard"),
			dataShards:         3,
			parityShards:       2,
			corruptShardsCount: 1,
		},
		{
			name:               "reconstruct with two missing shards",
			data:               []byte("testing erasure coding from 2 missing shard"),
			dataShards:         3,
			parityShards:       2,
			corruptShardsCount: 2,
		},
		{
			name:               "reconstruct with 3 missing shards",
			data:               []byte("testing erasure coding from 2 missing shard"),
			dataShards:         3,
			parityShards:       2,
			corruptShardsCount: 3,
		},
		{
			name:               "reconstruct with 4 missing shards",
			data:               []byte("testing erasure coding from 2 missing shard"),
			dataShards:         3,
			parityShards:       2,
			corruptShardsCount: 4,
		},
		{
			name:               "reconstruct with max missing shards",
			data:               bytes.Repeat([]byte("data is testing"), 50),
			dataShards:         5,
			parityShards:       3,
			corruptShardsCount: 3,
		},
		{
			name:               "Reconstruct TooMany Missing Shards: should fail ",
			data:               []byte("testing erasure coding from 4 missing shard"),
			dataShards:         5,
			parityShards:       3,
			corruptShardsCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coder, err := NewReedSolomonCoder(tt.dataShards, tt.parityShards)
			require.NoError(t, err)

			shards, err := coder.Erasure(tt.data)
			require.NoError(t, err)

			for i := range tt.corruptShardsCount {
				shards[i] = nil
			}

			var buf bytes.Buffer
			err = coder.Reconstruct(shards, &buf)

			if tt.corruptShardsCount > tt.parityShards {
				require.ErrorContains(t, err, "failed to reconstruct shards: too few shards given")
			} else {
				reconstructed := buf.Bytes()
				require.NoError(t, err)
				assert.Equal(t, tt.data, reconstructed)
			}
		})
	}
}
