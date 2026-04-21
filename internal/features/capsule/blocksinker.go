package capsule

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"

	"github.com/cespare/xxhash"
	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/engr-sjb/diogel/internal/dataredundancy"
	"github.com/engr-sjb/diogel/internal/message"
	"github.com/engr-sjb/diogel/internal/peererrors"
	"github.com/engr-sjb/diogel/internal/transport"
	"github.com/google/uuid"
)

// blockSinkEncoder accumulates data in write method into a block then encrypts the
// block which is then erasure coded into
type blockSinkEncoder struct {
	blockID         uint64
	capsuleManifest capsuleManifest
	blockBuf        []byte // todo: not sure if i need to make this an array of blockBuffSize or just use a slice of blockBuffSize. cause if the caller of our write method gives us a big slice more than our blockBufsize, append will grow the slice.
	erasureFunc     dataredundancy.ErasureFunc
	// rendezvousHasher
	capsuleID        uuid.UUID
	capsuleMasterKey []byte
	blockKey         [32]byte
	cCrypto          customcrypto.CCrypto
	remotePeers      []transport.RemotePeer
}

func NewBlockSinkEncoder(capsuleID uuid.UUID, capsuleMasterKey []byte, eF dataredundancy.ErasureFunc, rps []transport.RemotePeer) *blockSinkEncoder {
	return &blockSinkEncoder{
		blockID:          1,
		blockBuf:         make([]byte, 0, blockSinkBufSize),
		erasureFunc:      eF,
		capsuleID:        capsuleID,
		capsuleMasterKey: capsuleMasterKey,
		cCrypto:          customcrypto.NewCCrypto(),
		remotePeers:      rps,
	}
}

func (self *blockSinkEncoder) Write(data []byte) (n int, err error) {
	self.blockBuf = append(self.blockBuf, data...)

	for len(self.blockBuf) >= blockSinkBufSize {
		if err := self.processBlock(self.blockBuf[:blockSinkBufSize], false); err != nil {
			return 0, err
		}

		self.blockBuf = self.blockBuf[:blockSinkBufSize]
	}

	return len(data), nil
}

func (self *blockSinkEncoder) Close() error {
	//flushes remaining data in block
	if len(self.blockBuf) > 0 {
		log.Println("close running")
		if err := self.processBlock(self.blockBuf, true); err != nil {
			return err
		}
		self.blockBuf = self.blockBuf[:0]
	}
	return nil
}

func (self *blockSinkEncoder) GetManifest() {

}

func (self *blockSinkEncoder) processBlock(blockData []byte, isFinal bool) error {
	err := deriveBlockKey(
		self.blockID,
		self.capsuleMasterKey,
		&self.blockKey,
	)
	if err != nil {
		return err
	}

	encBlock, usedNonce, err := self.cCrypto.Cipher.Encrypt(
		self.blockKey[:],
		nil,
		blockData,
	)
	if err != nil {
		return err
	}

	shards, err := self.erasureFunc(encBlock)
	if err != nil {
		return peererrors.New(
			peererrors.ScopeInternalPeer,
			peererrors.CodeTodo,
			"failed to erasure code block", //Todo: better error handling message.
			err,
			featureCapsule,
		)
	}

	repairGroupID := uuid.New()
	shardStreamMessage := &message.CapsuleIncomingShardStream{
		CapsuleID:      self.capsuleID,
		ShardID:        uuid.New(),
		RepairGroupID:  repairGroupID,
		Nonce:          usedNonce,
		DataShardNum:   uint8(dataShardNum),
		ParityShardNum: uint8(parityShardNum),
	}

	// Todo: This work for now but would need too be changed if we add storage providers.
	for i := range shards {
		// Todo: this thing might have to change.
		bestRemotePeer := pickRemotePeer(
			self.blockID,
			i,
			self.remotePeers,
		)

		shardStreamMessage.ShardID = uuid.New()
		shardStreamMessage.Size = uint32(len(shards[i]))
		shardStreamMessage.IsFinal = isFinal

		n, err := bestRemotePeer.Send(
			shardStreamMessage,
			shards[i],
		)

		if n != len(shards[i]) {
			return peererrors.New(
				peererrors.ScopeInternalPeer,
				peererrors.CodeTodo,
				fmt.Sprintf(
					"failed to send shard: n'%d' is not equal to shard size '%d'",
					n,
					len(shards[i]),
				),
				nil,
				featureCapsule,
			)
		}

		if err != nil {
			return peererrors.New(
				peererrors.ScopeInternalPeer,
				peererrors.CodeTodo,
				"failed to send shard: err occurred",
				err,
				featureCapsule,
			)
		}

	}

	// Track repair group in manifest
	self.capsuleManifest.blocks = append(self.capsuleManifest.blocks, message.BlockManifest{
		RepairGroupID:  repairGroupID,
		DataShardNum:   uint8(dataShardNum),
		ParityShardNum: uint8(parityShardNum),
	})
	self.capsuleManifest.totalBlocks = self.blockID

	self.blockID++
	return nil
}

// Todo: For Storage Providers

// type shardMessage struct { //Todo: Move too message and this is imported here and send to guardians or storage providers.
// 	capsuleID     uuid.UUID // Todo: I think storage providers don't need to know this. Only Guardians.
// 	shardID       uuid.UUID
// 	repairGroupID uuid.UUID
// }

type shardMetaData struct {
	// blockID                      uuid.UUID
	capsuleID                    uuid.UUID
	shardID                      uuid.UUID
	repairGroupID                uuid.UUID
	nonce                        []byte
	hash                         [32]byte
	size                         uint32
	dataShardNum, parityShardNum uint8
}

// For the Guardians
type blockManifest struct {
	repairGroupID  uuid.UUID
	dataShardNum   uint8
	parityShardNum uint8
}

type capsuleManifest struct {
	capsuleID   uuid.UUID
	totalBlocks uint64
	blocks      []message.BlockManifest
}

func deriveBlockKey(blockID uint64, capsuleMasterKey []byte, blockKey *[32]byte) error {
	// h := sha256.New()
	// h.Write(self.capsuleMasterKey)
	// h.Write([]byte(fmt.Sprintf("block_%d", blockID)))
	// return h.Sum(nil) // Returns 32 bytes for AES-256

	h := sha256.New()
	_, err := h.Write(capsuleMasterKey)
	if err != nil {
		return err
	}

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], blockID)
	_, err = h.Write(buf[:])
	if err != nil {
		return err
	}
	h.Sum(blockKey[:0]) // Write directly into pre-allocated array
	return nil
}

type blockWriter struct {
	buf    []byte
	offSet int
	flush  func([]byte) error
}

func newBlockWriter(size int, flush func([]byte) error) blockWriter {
	return blockWriter{
		buf:   make([]byte, size),
		flush: flush,
	}
}

func (bw *blockWriter) Write(p []byte) (n int, err error) {
	totalWritten := 0

	for len(p) > 0 {
		n := copy(bw.buf[bw.offSet:], p)

		bw.offSet += n
		p = p[n:]
		totalWritten += n

		if len(bw.buf) == bw.offSet {
			if err := bw.flush(bw.buf); err != nil {
				return totalWritten, err
			}

			bw.offSet = 0
		}
	}

	return int(totalWritten), nil
}

func (bw *blockWriter) Close() error {
	if bw.offSet > 0 {
		if err := bw.flush(bw.buf[:bw.offSet]); err != nil {
			return err
		}
		bw.offSet = 0
	}

	return nil
}

func pickRemotePeer(block uint64, shard int, peers []transport.RemotePeer) transport.RemotePeer {
	//Todo: rethink how we distribute shards to peers and on what bases. Not sure.
	var bestScore uint64
	var bestPeer transport.RemotePeer
	for _, p := range peers {
		score := xxhash.Sum64String(
			fmt.Sprintf("%d:%d:%s", block, shard, p.ID().String()),
		)
		if bestPeer == nil || score > bestScore {
			bestPeer = p
			bestScore = score
		}
	}

	return bestPeer
}

func getBlockHash(block []byte) [32]byte {
	hash := sha256.Sum256(block)
	return hash
}
