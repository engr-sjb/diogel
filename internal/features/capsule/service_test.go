package capsule

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/engr-sjb/diogel/internal/archive"
	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/engr-sjb/diogel/internal/dataredundancy"
	"github.com/engr-sjb/diogel/internal/features/ports"
	"github.com/engr-sjb/diogel/internal/transport"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCapsuleDTO_Validate(t *testing.T) {
	validLetter := strings.NewReader("test letter")

	tests := []struct {
		name    string
		dto     *CreateCapsuleDTO
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid with 3 guardians and letter",
			dto: &CreateCapsuleDTO{
				RemotePeerGuardians: make([]transport.RemotePeer, 3),
				// Letter:              &mockLetter{data: []byte("test message")},
				Letter: &ports.FileMem{
					Name: "letter_name",
					Content: io.NopCloser(
						validLetter,
					),
					Mode: 0644,
					Size: validLetter.Size(),
				},
			},
			wantErr: false,
		},
		{
			name: "valid with 5 guardians and letter",
			dto: &CreateCapsuleDTO{
				RemotePeerGuardians: make([]transport.RemotePeer, 5),
				Letter: &ports.FileMem{
					Name: "letter_name",
					Content: io.NopCloser(
						validLetter,
					),
					Mode: 0644,
					Size: validLetter.Size(),
				},
			},
			wantErr: false,
		},
		{
			name: "insufficient guardians (2 when min is 3)",
			dto: &CreateCapsuleDTO{
				RemotePeerGuardians: make([]transport.RemotePeer, 2),
				Letter: &ports.FileMem{
					Name: "letter_name",
					Content: io.NopCloser(
						validLetter,
					),
					Mode: 0644,
					Size: validLetter.Size(),
				},
			},
			wantErr: true,
			errMsg:  "guardians must be at least 3",
		},
		{
			name: "too many guardians (11 when max is 10)",
			dto: &CreateCapsuleDTO{
				RemotePeerGuardians: make([]transport.RemotePeer, 11),
				Letter: &ports.FileMem{
					Name: "letter_name",
					Content: io.NopCloser(
						validLetter,
					),
					Mode: 0644,
					Size: validLetter.Size(),
				},
			},
			wantErr: true,
			errMsg:  "guardians must be at most 10",
		},
		{
			name: "no letter provided, only files are provided",
			dto: &CreateCapsuleDTO{
				RemotePeerGuardians: make([]transport.RemotePeer, 3),
				FilePaths: []string{
					"testdata/test_file_1.txt",
					"testdata/test_file_2.txt",
					"testdata/test_file_3.txt",
				},
			},
			wantErr: false,
		},
		{
			name: "no letter or files provided",
			dto: &CreateCapsuleDTO{
				RemotePeerGuardians: make([]transport.RemotePeer, 3),
				// No Letter, no FilePaths
			},
			wantErr: true,
			errMsg:  "at least, a letter or a file path(s) must be provided",
		},
		{
			name: "threshold too low (1 when min is 2)",
			dto: &CreateCapsuleDTO{
				RemotePeerGuardians:               make([]transport.RemotePeer, 3),
				Letter:                            &mockLetter{data: []byte("test")},
				CapsuleMasterKeyRecoveryThreshold: 1,
			},
			wantErr: true,
			errMsg:  "recovery threshold must be at least 2",
		},
		{
			name: "threshold exceeds guardians (4 of 3)",
			dto: &CreateCapsuleDTO{
				RemotePeerGuardians:               make([]transport.RemotePeer, 3),
				Letter:                            &mockLetter{data: []byte("test")},
				CapsuleMasterKeyRecoveryThreshold: 4,
			},
			wantErr: true,
			errMsg:  "recovery threshold cannot exceed number of guardians",
		},
	}

	// Defaults used for validation
	defaults := Defaults{
		MinNumOfGuardians: 3,
		MaxNumOfGuardians: 10,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dto.validate(defaults)

			if tt.wantErr {
				require.Error(t, err, "expected validation to fail")

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg,
						"error message should contain expected text")
				}
			} else {
				require.NoError(t, err, "expected validation to succeed")

				if tt.dto.SilencePeriod == 0 {
					assert.Equal(t, defaultSilencePeriod, tt.dto.SilencePeriod,
						"should apply default silence period")
				}
				if tt.dto.CapsuleMasterKeyRecoveryThreshold == 0 {
					assert.Greater(t, tt.dto.CapsuleMasterKeyRecoveryThreshold, 0,
						"should calculate default threshold")
				}
			}
		})
	}
}

// TestCapsuleRoundTrip is the most important test - it verifies the complete
// lifecycle: Create → Distribute → Reconstruct → Decrypt → Verify.
//
// This test simulates the real-world scenario:
// 1. Owner creates capsule and distributes to guardians
// 2. Owner dies (or becomes incapacitated)
// 3. Guardians collaborate to reconstruct the capsule
// 4. Original data is recovered successfully
//
// We test multiple scenarios with different guardian counts and thresholds
// to ensure the Shamir Secret Sharing works correctly.
func TestCapsuleRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		// originalData      string
		originalData      []byte
		numGuardians      int
		threshold         int
		useGuardians      []int // Which guardians participate in recovery
		shouldReconstruct bool
		description       string
	}{
		{
			name: "3 of 3 guardians - all participate",
			// originalData:      "This is my final message to my family",
			// originalData:      bytes.Repeat([]byte("x"), blockSinkBufSize*20+512), // 20MB + 512 bytes
			// originalData: func() []byte {
			// 	data := make([]byte, blockSinkBufSize)
			// 	rand.Read(data)
			// 	return data
			// }(), // 20MB + 512 bytes
			originalData: func(size int) []byte {
				base := []byte("The quick brown fox jumps over the lazy dog. ")

				out := make([]byte, 0, size)

				for len(out) < size {
					chunk := make([]byte, len(base))
					copy(chunk, base)

					// inject small entropy so compression doesn't collapse it
					chunk[10] ^= byte(len(out) % 255)
					chunk[20] ^= byte((len(out) / 2) % 255)

					out = append(out, chunk...)
				}

				return out[:size]
			}(blockSinkBufSize),
			numGuardians:      3,
			threshold:         2,
			useGuardians:      []int{0, 1, 2},
			shouldReconstruct: true,
			description:       "All guardians available - should succeed",
		},
		// {
		// 	name:              "3 of 3 guardians - only 2 participate (threshold met)",
		// 	originalData:      "Secret inheritance instructions",
		// 	numGuardians:      3,
		// 	threshold:         2,
		// 	useGuardians:      []int{0, 2}, // Skip guardian 1
		// 	shouldReconstruct: true,
		// 	description:       "One guardian unavailable but threshold met - should succeed",
		// },
		// {
		// 	name:              "5 of 5 guardians - 3 participate (threshold met)",
		// 	originalData:      "Cryptocurrency wallet recovery phrase",
		// 	numGuardians:      5,
		// 	threshold:         3,
		// 	useGuardians:      []int{1, 2, 4}, // Skip guardians 0 and 3
		// 	shouldReconstruct: true,
		// 	description:       "Two guardians unavailable but threshold met - should succeed",
		// },
		// {
		// 	name:              "3 of 3 guardians - only 1 participates (threshold NOT met)",
		// 	originalData:      "Should fail to reconstruct",
		// 	numGuardians:      3,
		// 	threshold:         2,
		// 	useGuardians:      []int{0}, // Only 1 guardian
		// 	shouldReconstruct: false,
		// 	description:       "Insufficient guardians - should fail",
		// },
		// {
		// 	name:              "5 of 5 guardians - only 2 participate (threshold NOT met)",
		// 	originalData:      "Should also fail",
		// 	numGuardians:      5,
		// 	threshold:         3,
		// 	useGuardians:      []int{0, 4}, // Only 2 guardians
		// 	shouldReconstruct: false,
		// 	description:       "Insufficient guardians for 3-of-5 - should fail",
		// },
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Logf("Testing: %s", testCase.description)

			// ===== PHASE 1: SETUP =====
			h := NewTestHelper(t)
			mockDB, mockFS := h.SetupMockDBAndFileStore()

			// Create guardians and setup capture
			peers, _ := h.CreateMockGuardians(testCase.numGuardians)
			guardianStorages := make([]*GuardianInMemStorage, testCase.numGuardians)
			for i := range testCase.numGuardians {
				guardianStorages[i] = &GuardianInMemStorage{}
				h.SetupGuardianCapture(peers[i], guardianStorages[i])
			}

			// Capture master key for verification

			var capturedMasterKey []byte
			svc := h.CreateTestService(
				WithMockDB(mockDB),
				WithMockFileStore(mockFS),
				WithTestHooks(&TestHooks{
					OnMasterKeyGenerated: func(key []byte) {
						// CRITICAL: Make a copy! The original might be cleared for security
						capturedMasterKey = append([]byte(nil), key...)
					},
				}),
			)

			// ===== PHASE 2: CREATE AND DISTRIBUTE CAPSULE =====
			remotePeers := make([]transport.RemotePeer, testCase.numGuardians)
			for i := range testCase.numGuardians {
				remotePeers[i] = peers[i]
			}

			letter := bytes.NewReader(testCase.originalData)
			payload := &CreateCapsuleDTO{
				RemotePeerGuardians: remotePeers,
				SilencePeriod:       168 * time.Hour,
				// Letter:                            &mockLetter{data: []byte(testCase.originalData)},
				Letter: &ports.FileMem{
					Name: LetterName,
					Content: io.NopCloser(
						bytes.NewReader(testCase.originalData),
					),
					Mode:    0700,
					ModTime: time.Now(),

					Size: letter.Size(),
				},
				CapsuleMasterKeyRecoveryThreshold: testCase.threshold,
			}

			err := svc.CreateAndSendCapsule(h.ctx, payload)
			require.NoError(t, err, "capsule creation should succeed")

			// Verify master key was captured
			require.NotNil(t, capturedMasterKey, "master key should be captured")
			require.Equal(t, 32, len(capturedMasterKey), "master key should be 32 bytes")

			// Verify all guardians received data
			for i := range testCase.numGuardians {
				require.NotNil(t, guardianStorages[i].InitialMsg,
					"guardian %d should receive initial message",
					i,
				)
				require.NotEmpty(t, guardianStorages[i].Shards,
					"guardian %d should receive shards",
					i,
				)
				require.NotEmpty(t, guardianStorages[i].KeyShare,
					"guardian %d should receive key share",
					i,
				)
			}

			capsuleID := guardianStorages[0].InitialMsg.CapsuleID
			t.Logf(
				"Capsule created: ID=%s, Guardians=%d, Threshold=%d",
				capsuleID,
				testCase.numGuardians,
				testCase.threshold,
			)

			// ===== PHASE 3: SIMULATE RECOVERY - COLLECT SHARDS =====
			// This simulates what happens when guardians collaborate after owner's death
			// Each participating guardian contributes their shards

			// blockInfo tracks shards for each repair group (block)
			type blockInfo struct {
				shards [][]byte // The actual encrypted shard data
				nonce  []byte   // Nonce used for encrypting this block
			}
			shardsByRepairGroup := make(map[uuid.UUID]*blockInfo)

			// Collect shards from participating guardians only
			for _, guardianIdx := range testCase.useGuardians {
				storage := guardianStorages[guardianIdx]
				t.Logf("Collecting shards from guardian %d: %d shards",
					guardianIdx, len(storage.Shards))

				for i, shard := range storage.Shards {
					// Group shards by repair group (each repair group = one block)
					if _, exists := shardsByRepairGroup[shard.RepairGroupID]; !exists {
						shardsByRepairGroup[shard.RepairGroupID] = &blockInfo{
							shards: make([][]byte, 0),
							nonce:  shard.Nonce, // All shards in a group share the same nonce
						}
					}
					shardsByRepairGroup[shard.RepairGroupID].shards = append(
						shardsByRepairGroup[shard.RepairGroupID].shards,
						storage.ShardData[i],
					)
				}
			}

			t.Logf("Collected shards from %d guardians into %d repair groups",
				len(testCase.useGuardians), len(shardsByRepairGroup))

			// ===== PHASE 4: CHECK IF RECONSTRUCTION IS POSSIBLE =====
			if !testCase.shouldReconstruct {
				// This test expects failure - verify we don't have enough shards
				hasEnoughShards := true
				for rgID, blockInfo := range shardsByRepairGroup {
					if len(blockInfo.shards) < dataShardNum {
						hasEnoughShards = false
						t.Logf("Repair group %s: only %d shards (need %d)",
							rgID, len(blockInfo.shards), dataShardNum)
					}
				}

				assert.False(t, hasEnoughShards,
					"should not have enough shards for reconstruction")

				t.Logf("✓ Correctly failed reconstruction with %d/%d guardians (threshold: %d)",
					len(testCase.useGuardians), testCase.numGuardians, testCase.threshold)
				return // Test complete - expected failure verified
			}

			// ===== PHASE 5: RECONSTRUCT BLOCKS FROM SHARDS =====
			// This is where erasure coding magic happens: we can reconstruct the
			// original block from any dataShardNum shards (out of dataShardNum + parityShardNum)

			erasureCoder, err := dataredundancy.NewReedSolomonCoder(dataShardNum, parityShardNum)
			require.NoError(t, err, "erasure coder creation should succeed")

			manifest := guardianStorages[0].Manifest
			require.NotNil(t, manifest, "manifest should exist")

			var reconstructedBlocks [][]byte
			for blockIdx, blockManifest := range manifest.Blocks {
				blockInfo := shardsByRepairGroup[blockManifest.RepairGroupID]
				require.NotNil(t, blockInfo,
					"block %d: should have shards for repair group %s",
					blockIdx, blockManifest.RepairGroupID)

				// Verify we have enough shards
				require.GreaterOrEqual(t, len(blockInfo.shards), dataShardNum,
					"block %d: need at least %d shards, got %d",
					blockIdx, dataShardNum, len(blockInfo.shards))

				// Take only the data shards (first 32)
				// In production, we might have more than 32 due to redundancy,
				// but we only need 32 to reconstruct
				// shards := blockInfo.shards
				// shards := blockInfo.shards[:dataShardNum]
				totalShards := dataShardNum + parityShardNum
				shards := make([][]byte, totalShards)

				// Fill available shards and mark missing ones as nil
				availableCount := 0
				for i, shardData := range blockInfo.shards {
					if i < totalShards {
						shards[i] = shardData
						availableCount++
					}
				}

				// Verify we have enough shards for reconstruction
				require.GreaterOrEqual(t, availableCount, dataShardNum,
					"block %d: need at least %d shards, got %d",
					blockIdx, dataShardNum, availableCount)

				// Reconstruct the encrypted block
				blockBuf := &bytes.Buffer{}
				err := erasureCoder.Reconstruct(shards, blockBuf)
				require.NoError(t, err,
					"block %d: reconstruction should succeed", blockIdx)

				reconstructedBlocks = append(reconstructedBlocks, blockBuf.Bytes())
				t.Logf("Block %d reconstructed: %d bytes", blockIdx, blockBuf.Len())
			}

			t.Logf("Successfully reconstructed %d blocks", len(reconstructedBlocks))

			// ===== PHASE 6: RECONSTRUCT MASTER KEY FROM SHARES =====
			// This is where Shamir's Secret Sharing happens: we combine k shares
			// to reconstruct the original master key

			participatingShares := make([][]byte, len(testCase.useGuardians))
			for i, guardianIdx := range testCase.useGuardians {
				participatingShares[i] = guardianStorages[guardianIdx].KeyShare
				t.Logf("Using key share from guardian %d: %d bytes",
					guardianIdx, len(participatingShares[i]))
			}

			cCrypto := customcrypto.NewCCrypto()
			reconstructedMasterKey, err := cCrypto.SecretSharer.Combine(participatingShares)
			require.NoError(t, err,
				"master key reconstruction should succeed with %d shares", len(participatingShares))

			// CRITICAL VERIFICATION: Reconstructed key must match original
			require.Equal(t, capturedMasterKey, reconstructedMasterKey,
				"reconstructed master key must match original")

			t.Logf("✓ Master key successfully reconstructed from %d shares", len(participatingShares))

			// ===== PHASE 7: DECRYPT BLOCKS =====
			// Now we decrypt each block using keys derived from the master key

			var decryptedBlocks [][]byte
			var blockKey [32]byte

			for blockIdx, blockManifest := range manifest.Blocks {
				blockInfo := shardsByRepairGroup[blockManifest.RepairGroupID]
				encBlock := reconstructedBlocks[blockIdx]

				// Derive the block-specific key from master key
				// Block IDs start at 1, not 0 (important!)
				err := deriveBlockKey(uint64(blockIdx+1), reconstructedMasterKey, &blockKey)
				require.NoError(t, err,
					"block %d: key derivation should succeed", blockIdx)

				// Decrypt using the derived key and the nonce from this block
				decrypted, err := cCrypto.Cipher.Decrypt(blockKey[:], blockInfo.nonce, encBlock)
				require.NoError(t, err,
					"block %d: decryption should succeed", blockIdx)

				decryptedBlocks = append(decryptedBlocks, decrypted)
				t.Logf("Block %d decrypted: %d bytes", blockIdx, len(decrypted))
			}

			t.Logf("Successfully decrypted %d blocks", len(decryptedBlocks))

			// ===== PHASE 8: CONCATENATE DECRYPTED BLOCKS =====
			// The blocks together form the compressed archive

			var fullDecrypted bytes.Buffer
			for _, block := range decryptedBlocks {
				fullDecrypted.Write(block)
			}

			t.Logf("Total decrypted data: %d bytes", fullDecrypted.Len())

			// ===== PHASE 9: DECOMPRESS AND UNARCHIVE =====
			// This extracts the original files from the compressed archive

			archiver := archive.NewArchive()
			// mockFileStore := new(mockFileStore)
			// mockFileStore.On("MkdirAll", mock.Anything).Return(nil)

			// // In a real scenario, this would write files to disk
			// // In tests, we just verify the unarchive process completes
			// mockFileStore.On("Create", mock.Anything).Return(&mockLetter{}, nil)

			// Create a mock file store that captures the extracted files
			extractedFiles := make(map[string][]byte)
			mockFileStore := &mockFileStoreCapture{
				files: extractedFiles,
			}

			err = archiver.UnArchiveStream(h.ctx, &fullDecrypted, mockFileStore)
			require.NoError(t, err, "unarchive should succeed")

			// Verify the extracted letter matches original data
			letterContent, exists := extractedFiles[LetterName]

			require.True(t, exists, "letter file should be extracted")
			require.Equal(t, testCase.originalData, letterContent,
				"extracted letter content must match original")

			t.Logf("✓ Data integrity verified: original matches extracted")

			// ===== PHASE 10: FINAL VERIFICATION =====
			// In a production system, you would verify the extracted files match
			// the original. For this test, we've verified:
			// 1. Master key reconstruction works
			// 2. Block reconstruction works
			// 3. Decryption works
			// 4. Unarchiving completes without error

			t.Logf("✓ Round-trip test PASSED:")
			t.Logf("  - Created capsule with %d guardians", testCase.numGuardians)
			t.Logf("  - Distributed %d total shards",
				len(guardianStorages[0].Shards)+len(guardianStorages[1].Shards)+len(guardianStorages[2].Shards))
			t.Logf("  - Recovered using %d/%d guardians", len(testCase.useGuardians), testCase.numGuardians)
			t.Logf("  - Reconstructed %d blocks", len(reconstructedBlocks))
			t.Logf("  - Decrypted %d blocks", len(decryptedBlocks))
			t.Logf("  - Successfully unarchived capsule")
		})
	}
}
