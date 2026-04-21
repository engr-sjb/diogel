/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package capsule

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/engr-sjb/diogel/internal/archive"
	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/engr-sjb/diogel/internal/dataredundancy"
	"github.com/engr-sjb/diogel/internal/features"
	"github.com/engr-sjb/diogel/internal/features/ports"
	"github.com/engr-sjb/diogel/internal/message"
	"github.com/engr-sjb/diogel/internal/peererrors"
	"github.com/engr-sjb/diogel/internal/serialize"
	"github.com/engr-sjb/diogel/internal/shared/database"
	"github.com/engr-sjb/diogel/internal/transport"
	"github.com/google/uuid"
)

var (
	ErrInvalidGuardiansCount    = errors.New("invalid guardians count")
	ErrInvalidCreateCapsuleData = errors.New("invalid create capsule data")
)

const (
	featureCapsule features.FeatureLocation = "capsule"

	LetterName = "capsule_letter.txt"

	defaultSilencePeriod = 168 * time.Hour // 7 days

	// It is 1 because we will always have one file at min. The Message file is default file. Added files are extra data added. So we can create a capsule with this one file or added additional files to be in the capsule.
	defaultFilesEntriesSize = 1

	// buffSize uint32 = 32 * 1024

	bufSize = 1 << 15 //32kb

	dataShardNum   int = 32
	parityShardNum int = 22

	blockSinkBufSize = 1 << 20 // 1mb
	maxShardSize     = blockSinkBufSize / (dataShardNum + parityShardNum)
)

type servicer interface {
	CreateAndSendCapsule(ctx context.Context, payload *CreateCapsuleDTO) error
	ReceiveCapsuleStream(msgCtx context.Context, remotePeer transport.RemotePeer, msg *message.CapsuleIncomingStream) error
	ReceiveContinueCapsuleStream(
		ctx context.Context, remotePeer transport.RemotePeer, msg message.CapsuleReStream,
	) error
	ReceiveReCapsuleStream(
		ctx context.Context, remotePeer transport.RemotePeer, msg message.CapsuleReStream,
	) error
	GetDefaults() Defaults // GetDefaults retrieves default values of this service.
}

var _ servicer = (*service)(nil)

type OnFindRemotePeers func([]customcrypto.PublicKeyStr, []*transport.RemotePeer) // todo: do it a different way by send the remote peers into the method in the orch.

type Defaults struct {
	MinNumOfGuardians uint
	MaxNumOfGuardians uint

	MasterCapsuleKeySplitThreshold uint
}

// TestHooks hold all Hooks needed for tests that are generated internally and need for tests that we need multiple moving parts for verification.
type TestHooks struct {
	OnMasterKeyGenerated func([]byte)
}

type ServiceConfig struct {
	// NOTICE IMPORTANT: When you add a field, ALWAYS check if it is it's default value in its contractor func.

	*Defaults
	Ctx      context.Context
	Shutdown *sync.WaitGroup
	// Todo: take in our peer ID.
	PeerID     uuid.UUID
	PrivateKey []byte
	PublicKey  []byte
	//todo: we need to find a way to
	DBStore             dbStorer
	FileStore           objectStorer
	Serialize           serialize.Serializer
	CCrypto             customcrypto.CCrypto
	Archive             archive.Archiver
	NewErasureCoderFunc dataredundancy.NewErasureCoderFunc
	TestHooks           *TestHooks
	// erasureCode dataredundancy.ErasureCoder
}

type service struct {
	*ServiceConfig
}

func NewService(cfg *ServiceConfig) *service {
	// NOTICE IMPORTANT: Check if all fields on cfg are not their default value before use.
	switch {
	case cfg == nil:
		log.Fatal("ServiceConfig cannot be nil")
	case cfg.Ctx == nil:
		log.Fatal("Context cannot be nil")
	case cfg.Shutdown == nil:
		log.Fatal("Shutdown cannot be nil")
	case cfg.MinNumOfGuardians < 3:
		log.Fatal("Minimum number of guardians must be 3")
	case cfg.PrivateKey == nil:
		log.Fatal("PrivateKey cannot be nil")
	case cfg.PublicKey == nil:
		log.Fatal("PublicKey cannot be nil")
	case cfg.DBStore == nil:
		log.Fatal("DBStore cannot be nil")
	case cfg.FileStore == nil:
		log.Fatal("FileStore cannot be nil")
	case cfg.CCrypto.Cipher == nil || cfg.CCrypto.DeriveKey == nil || cfg.CCrypto.GenerateKeyPair == nil:
		log.Fatal("CCrypto cannot be nil")
	case cfg.Serialize == nil:
		log.Fatal("Serialize cannot be nil")
	case cfg.Archive == nil:
		log.Fatal("Archive cannot be nil")
	case cfg.NewErasureCoderFunc == nil:
		log.Fatal("NewErasureCoder cannot be nil")
	}

	return &service{
		ServiceConfig: cfg,
	}
}

func (s *service) CreateAndSendCapsule(ctx context.Context, payload *CreateCapsuleDTO) error {
	err := payload.validate(
		Defaults{
			MinNumOfGuardians: s.MinNumOfGuardians,
			MaxNumOfGuardians: s.MaxNumOfGuardians,
		},
	)
	if err != nil {
		return err
	}

	numOfFiles, err := payload.GetNumOfFiles()
	if err != nil {
		return err
	}

	files := make([]ports.File, numOfFiles)
	if len(payload.FilePaths) > 0 {
		err := s.FileStore.Open(
			localDisk,
			payload.FilePaths,
			files,
		)
		if err != nil {
			return peererrors.New(
				peererrors.ScopeLocalPeer,
				peererrors.CodeTodo,
				"failed to open files(s)",
				err,
				featureCapsule,
			)
		}
	}

	if payload.Letter != nil {
		files[len(files)-1] = payload.Letter
	}
	// else if payload.Letter == nil { //todo: this can be deleted as validate handles it but think of it.
	// 	return peererrors.New(
	// 		peererrors.CodeLocalPeerError,
	// 		"letter cannot be nil",
	// 		nil,
	// 		featureCapsule,
	// 	)
	// }

	remotePeersIDs := make([]uuid.UUID, len(payload.RemotePeerGuardians))
	for i := range payload.RemotePeerGuardians {
		remotePeersIDs[i] = payload.RemotePeerGuardians[i].ID()
	}

	capsuleID := uuid.New()

	// capsuleMasterKey
	capsuleMasterKey := make([]byte, 32)
	if _, err := rand.Read(capsuleMasterKey); err != nil {
		return peererrors.New(
			peererrors.ScopeInternalPeer,
			peererrors.CodeTodo,
			"failed to generate master key",
			err,
			featureCapsule,
		)
	}

	//Test support to capture capsule key
	if s.TestHooks != nil && s.TestHooks.OnMasterKeyGenerated != nil {
		s.TestHooks.OnMasterKeyGenerated(capsuleMasterKey)
	}

	// split capsuleMasterKey for guardians
	masterKeySplitShares, err := s.CCrypto.SecretSharer.Split(
		capsuleMasterKey,
		len(payload.RemotePeerGuardians), // Todo: Tricky thing. as we know, we can have have active peers which reduces the number of peers. so now we split the key by the active or total umber of guardians? if so we need to make sure we reach them later with their respective splits. What if we never reach them? this breaks the whole thing.
		payload.CapsuleMasterKeyRecoveryThreshold,
	)
	if err != nil {
		return peererrors.New(
			peererrors.ScopeLocalPeer,
			peererrors.CodeTodo,
			"failed to split master key shares",
			err,
			featureCapsule,
		)
	}

	// STEP:we create msg and send, if we get the peer, we know they are active. if we error and don't get minimum number, we send another message to cancel for the peers that were sent to.
	msg := &message.CapsuleIncomingStream{
		CapsuleID:            capsuleID,
		GuardiansIDs:         remotePeersIDs,
		HeartbeatGracePeriod: payload.SilencePeriod,
		ShardSize:            uint16(maxShardSize),
		KeyShareSize:         uint8(len(masterKeySplitShares[0])),
	}

	msgErr := make([]error, len(payload.RemotePeerGuardians)) //todo: Might have to make this implement the error interface or something else. i don't want to make allocation here again.

	activeRemotePeerCount := 0
	for i := range payload.RemotePeerGuardians {
		_, err := payload.RemotePeerGuardians[i].Send(msg, nil)
		if err != nil {
			// TODO: We need to do some thinking here to make it resilient. What if we send to
			//Todo: Cause we are going to shard, we need a min number of shards from a number of peers, but we need min num off peers to be greater than shard parity or such. so we need to rethink the idea of send to only minimum peers num.
			//Todo: if we error and don't get minimum number, we send another message to cancel for the peers that were sent to. We need to do that here.

			// Notice: The idea is to still send a msg if the number of remaining peers are still greater than the minNum of guardians. If they are, we send the message to the remaining and add the errors to the errors slice which we return to the call so they can retry later or such. I dot know yet. .

			msgErr = append(msgErr, peererrors.New(
				peererrors.ScopeLocalPeer,
				peererrors.CodeTodo,
				fmt.Sprintf(
					"failed to send capsule to a remote peer with ID: %s",
					payload.RemotePeerGuardians[i].ID(),
				),
				err,
				featureCapsule,
			))

			continue
		}

		payload.RemotePeerGuardians[activeRemotePeerCount] = payload.RemotePeerGuardians[i]
		activeRemotePeerCount++
	}

	if int(s.MinNumOfGuardians) > activeRemotePeerCount {
		//Todo: if we error and don't get minimum number, we send another message to cancel for the peers that were sent to. We need to do that here. We send a cancel message to the active that are below the minimum num so they aren't sitting waiting.

		return peererrors.New(
			peererrors.ScopeLocalPeer,
			peererrors.CodeTodo,
			fmt.Sprintf(
				"failed to send capsule to a remote peer and since the number of remaining peers are less than the min number of guardians, we can't send the message to the remaining peers. insufficient active guardians: got %d, need %d",
				activeRemotePeerCount,
				s.MinNumOfGuardians,
			),
			// msgErr,
			nil, //Todo: Rethink the error here. Maybe implement error interface for msgErr.
			featureCapsule,
		)
	}

	payload.RemotePeerGuardians = payload.RemotePeerGuardians[:activeRemotePeerCount]
	//TODO: We need to find a way to send the msgErrs(need to change the name since i would use it for non breaking errors) back to the caller. Might have to send a pointer in here which is checked later or return an err slice. Not sure.

	erasureCoder, err := s.NewErasureCoderFunc(
		dataShardNum,
		parityShardNum,
	)
	if err != nil {
		return peererrors.New(
			peererrors.ScopeInternalPeer,
			peererrors.CodeTodo,
			"failed to create a new erasure coder",
			err,
			featureCapsule,
		)
	}

	blockSinker := NewBlockSinkEncoder(
		capsuleID,
		capsuleMasterKey,
		erasureCoder.Erasure,
		payload.RemotePeerGuardians,
	)

	err = s.Archive.ArchiveStream(ctx, files, blockSinker)
	if err != nil {
		return peererrors.New(
			peererrors.ScopeLocalPeer,
			peererrors.CodeTodo,
			"failed to compress files",
			err,
			featureCapsule,
		)
	}

	// Todo: We can check block for block manifest details i think.
	manifestMsg := &message.CapsuleIncomingManifestStream{
		CapsuleID:   capsuleID,
		TotalBlocks: blockSinker.capsuleManifest.totalBlocks,
		Blocks:      blockSinker.capsuleManifest.blocks,
	}

	for i := range payload.RemotePeerGuardians {
		_, err := payload.RemotePeerGuardians[i].Send(manifestMsg, nil)
		if err != nil {
			return peererrors.New(
				peererrors.ScopeLocalPeer,
				peererrors.CodeTodo,
				fmt.Sprintf(
					"failed to send manifest to guardian with ID: %s",
					payload.RemotePeerGuardians[i].ID(),
				),
				err,
				featureCapsule,
			)
		}
	}

	var capsuleKeyShareMsg *message.CapsuleMasterKeyShare

	for i, rp := range payload.RemotePeerGuardians {
		capsuleKeyShareMsg = &message.CapsuleMasterKeyShare{
			CapsuleID:   capsuleID,
			TotalShares: uint16(len(masterKeySplitShares)),
			// ShareNumber: uint16(i),
			// Share:     make([]byte, len(masterKeySplitShares[i])),
			ThresholdShares: uint8(payload.CapsuleMasterKeyRecoveryThreshold),
		}
		n, err := rp.Send(capsuleKeyShareMsg, masterKeySplitShares[i])
		if n != len(masterKeySplitShares[i]) {
			return peererrors.New(
				peererrors.ScopeInternalPeer,
				peererrors.CodeTodo,
				fmt.Sprintf(
					"sent data is not equal to masterKeySplitShares size: sentData=%d, masterKeySplitShares=%d",
					n,
					len(masterKeySplitShares[i]),
				),
				nil,
				featureCapsule,
			)
		}
		if err != nil {
			return peererrors.New(
				peererrors.ScopeInternalPeer,
				peererrors.CodeTodo,
				"failed to send masterKeySplitShares to remote peers",
				err,
				featureCapsule,
			)
		}
	}

	// todo: we need a way to check our connected peers and find the guardians remote peer conn. if we have them, then send a message and after send the file stream. if not we dial them. but i think we just to expose a callback that is injected into the service that tries to retrieve a slice of public keys and returns a slice of remote peers. internally, it searches in connected peers map in peer and retrieve them. if not it sends the public key for discovery slice to find their addresses and then send it back to peer orch to then send to transport for dialing and adding to connected peers if no error.

	// in this case, we send master key share. and the encrypted data send.
	/*
		todo:
			-we send message first.
			- we send master key shares to all the respective guardian
			- we send the encrypted capsule to all the respective guardian if they aren't using the remote file storer option.
			- EXTRA FEATURE: remote encrypted file storer.

	*/

	return nil // Todo: We might have to return msgErrors here as a guarantee to tell the caller that we did send but we did have these ones fail. I think this has to return a flag to tell if we got the minimum or we didn't send at all. not sure yet.
}

func (s *service) ReceiveCapsuleStream(
	ctx context.Context, remotePeer transport.RemotePeer, msg *message.CapsuleIncomingStream,
) error {

	// Todo: handle failure on reversing db actions on failure at various steps.
	// todo: I might have to rethink about the value. Not sure capsule is right value here.Maybe change db to SQLite.

	// defer func() {
	// 	_, _ = s.FileStore.Delete(
	// 		// localDisk,
	// 		fmt.Sprintf(
	// 			"%s/%s",
	// 			remotePeer.PublicKeyStr(),
	// 			msg.CapsuleID.String(),
	// 		),
	// 	)
	// 	}()

	if msg == nil {
		return peererrors.New(
			peererrors.ScopeInternalPeer,
			peererrors.CodeTodo,
			"received nil capsule stream message",
			nil,
			featureCapsule,
		)
	}

	//- we create the metadata in our database to hold info on the capsule.
	// - create temp metadata for current in stream capsule for continuation, and shard organization.
	err := s.DBStore.createOrUpdate(
		// todo: I might have to rethink about the value. Not sure capsule is right value here.
		database.CollCapsules,
		msg.CapsuleID.String(),
		// todo: I might have to rethink about the value. Not sure capsule is right value here.
		&capsule{
			OwnerID:     remotePeer.ID(),
			GuardianIDs: msg.GuardiansIDs,
			CreatedAt:   msg.CreatedAt,
			ReceivedAt:  time.Now(),
			IsComplete:  false,
		},
	)
	if err != nil {
		return peererrors.New(
			peererrors.ScopeInternalPeer,
			peererrors.CodeTodo,
			fmt.Sprintf(
				"failed to create temp metadata for incoming capsule stream: CapsuleID '%s' by RemotePeerID '%s' ",
				msg.CapsuleID.String(),
				remotePeer.ID(),
			),
			err,
			featureCapsule,
		)
	}

	// - Now handle capsule shards.
	var (
		receivedShardMetaDataMsg message.CapsuleIncomingShardStream
		receivedShardData        = make([]byte, msg.ShardSize)
		shardsReceived           = 0    // IMPROVEMENT: Track progress
		maxShards                = 1000 // SECURITY: Prevent DoS attacks
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
		}

		if shardsReceived >= maxShards {
			return peererrors.New(
				peererrors.ScopeInternalPeer,
				peererrors.CodeTodo,
				fmt.Sprintf("exceeded maximum shards limit: %d /Possible DoS attack/", maxShards),
				nil,
				featureCapsule,
			)
		}

		nShardMsg, err := remotePeer.Receive(
			&receivedShardMetaDataMsg,
			receivedShardData,
		)
		if err != nil {
			return peererrors.New(
				peererrors.ScopeInternalPeer,
				peererrors.ErrBadRequest,
				"failed to receive capsule chunk",
				err,
				featureCapsule,
			)
		}

		if receivedShardMetaDataMsg.CapsuleID != msg.CapsuleID {
			return peererrors.New(
				peererrors.ScopeInternalPeer,
				peererrors.CodeTodo,
				fmt.Sprintf(
					"capsule ID mismatch: expected %s, got %s",
					msg.CapsuleID.String(),
					receivedShardMetaDataMsg.CapsuleID.String(),
				),
				nil,
				featureCapsule,
			)
		}

		if nShardMsg != int(receivedShardMetaDataMsg.Size) {
			return peererrors.New(
				peererrors.ScopeInternalPeer,
				peererrors.CodeTodo,
				fmt.Sprintf(
					"received shard size mismatch: got %d, expected %d from remote peer with ID: %s",
					nShardMsg,
					receivedShardMetaDataMsg.Size,
					remotePeer.ID().String(),
				),
				nil,
				featureCapsule)
		}

		if receivedShardMetaDataMsg.DataShardNum == 0 || receivedShardMetaDataMsg.ParityShardNum == 0 {
			return peererrors.New(
				peererrors.ScopeInternalPeer,
				peererrors.CodeTodo,
				"invalid erasure coding parameters: data or parity shard count is zero",
				nil,
				featureCapsule,
			)
		}

		// - Store shard and the
		// Compute hash for CAS storage
		//todo: saving shard in CAS and db should be transactional, if one fails the other has too be reversed.
		shardHash := sha256.Sum256(receivedShardData[:nShardMsg])
		err = s.FileStore.SaveCAS(shardHash, receivedShardData[:nShardMsg])
		if err != nil {
			return peererrors.New(
				peererrors.ScopeInternalPeer,
				peererrors.CodeTodo,
				"failed to save received capsule shard to CAS",
				err,
				featureCapsule,
			)
		}

		// Store shard metadata in database
		shardMeta := shardMetaData{
			capsuleID:      msg.CapsuleID,
			shardID:        receivedShardMetaDataMsg.ShardID,
			repairGroupID:  receivedShardMetaDataMsg.RepairGroupID,
			hash:           shardHash,
			nonce:          receivedShardMetaDataMsg.Nonce,
			size:           receivedShardMetaDataMsg.Size,
			dataShardNum:   receivedShardMetaDataMsg.DataShardNum,
			parityShardNum: receivedShardMetaDataMsg.ParityShardNum,
		}
		err = s.DBStore.createOrUpdate(
			database.CollCapsulesActiveShards,
			receivedShardMetaDataMsg.ShardID.String(),
			shardMeta,
		)
		if err != nil {
			//Todo: Should delete shard

			return peererrors.New(
				peererrors.ScopeInternalPeer,
				peererrors.CodeTodo,
				"failed to store shard metadata",
				err,
				featureCapsule,
			)
		}

		if receivedShardMetaDataMsg.IsFinal {
			err = s.DBStore.createOrUpdate(
				database.CollCapsules,
				msg.CapsuleID.String(),
				&capsule{
					OwnerID:     remotePeer.ID(),
					GuardianIDs: msg.GuardiansIDs,
					IsComplete:  true,
					CompletedAt: time.Now(),
				},
			)
			if err != nil {
				return peererrors.New(
					peererrors.ScopeInternalPeer,
					peererrors.CodeTodo,
					fmt.Sprintf(
						"failed ailed to mark capsule as complete for incoming capsule stream: CapsuleID '%s' by RemotePeerID '%s' ",
						msg.CapsuleID.String(),
						remotePeer.ID(),
					),
					err,
					featureCapsule,
				)
			}

			break
		}

		shardsReceived++
	}

	// - Now handle capsule manifest.
	// This tells us all the repair group IDs we need to look for during recovery
	var manifestMsg message.CapsuleIncomingManifestStream
	_, err = remotePeer.Receive(&manifestMsg, nil)
	if err != nil {
		return peererrors.New(
			peererrors.ScopeInternalPeer,
			peererrors.CodeTodo,
			"failed to receive capsule manifest",
			err,
			featureCapsule,
		)
	}

	if manifestMsg.CapsuleID != msg.CapsuleID {
		return peererrors.New(
			peererrors.ScopeInternalPeer,
			peererrors.CodeTodo,
			fmt.Sprintf(
				"manifest capsule ID mismatch: expected %s, got %s",
				msg.CapsuleID.String(),
				manifestMsg.CapsuleID.String(),
			),
			nil,
			featureCapsule,
		)
	}

	err = s.DBStore.createOrUpdate(
		database.CollCapsuleManifests,
		manifestMsg.CapsuleID.String(),
		&manifestMsg,
	)
	if err != nil {
		return peererrors.New(
			peererrors.ScopeInternalPeer,
			peererrors.CodeTodo,
			"failed to store capsule manifest",
			err,
			featureCapsule,
		)
	}

	// - Now handle key share reception
	var (
		receivedKeyShareMsg  message.CapsuleMasterKeyShare
		receivedKeyShareData = make([]byte, msg.KeyShareSize)
	)

	nKeyShareMsg, err := remotePeer.Receive(&receivedKeyShareMsg, receivedKeyShareData)
	if nKeyShareMsg != int(msg.KeyShareSize) {
		return peererrors.New(
			peererrors.ScopeInternalPeer,
			peererrors.CodeTodo,
			fmt.Sprintf(
				"received keyShareData size mismatch: got %d, expected %d from remote peer with ID: %s",
				nKeyShareMsg,
				msg.KeyShareSize,
				remotePeer.ID().String(),
			),
			nil,
			featureCapsule)
	}
	if err != nil {
		return peererrors.New(
			peererrors.ScopeInternalPeer,
			peererrors.CodeTodo,
			"failed to receive capsule key share",
			err,
			featureCapsule,
		)
	}

	masterKeyShare := &masterKeyShare{
		CapsuleID:       msg.CapsuleID,
		Share:           receivedKeyShareData[:nKeyShareMsg],      // The actual Shamir share
		TotalShares:     int(receivedKeyShareMsg.TotalShares),     // e.g., 3 total guardians
		ThresholdShares: int(receivedKeyShareMsg.ThresholdShares), // e.g., need 2 to decrypt
	}

	err = s.DBStore.createOrUpdate(
		database.CollKeyShares,
		msg.CapsuleID.String(),
		masterKeyShare,
	)
	if err != nil {
		return peererrors.New(
			peererrors.ScopeInternalPeer,
			peererrors.CodeTodo,
			"failed to store master key share",
			err,
			featureCapsule,
		)
	}

	err = s.DBStore.createOrUpdate(
		database.CollCapsules,
		msg.CapsuleID.String(),
		&capsule{
			OwnerID:                  remotePeer.ID(),
			GuardianIDs:              msg.GuardiansIDs,
			IsKeyMasterShareReceived: true,
			CompletedAt:              time.Now(),
		},
	)

	return nil
}

func (s *service) ReceiveContinueCapsuleStream(
	ctx context.Context, remotePeer transport.RemotePeer, msg message.CapsuleReStream,
) error {
	panic("unimplemented")
}

func (s *service) ReceiveReCapsuleStream(
	ctx context.Context, remotePeer transport.RemotePeer, msg message.CapsuleReStream,
) error {
	panic("unimplemented")
}

// GetDefaults retrieves default values of this service.
func (s service) GetDefaults() Defaults {
	return *s.Defaults
}

func (s *service) deriveShardKey(capsuleMasterKey []byte, shardIndex int, derivedKey []byte) error {
	//Todo: maybe do research on how to get string bytes as i know i would make it a byte slice for salt.
	info := fmt.Sprintf("capsule-shard-%d", shardIndex)
	// todo: we need to make derive take in a derived key as input to avoid it escaping as returned.
	derivedKey, _, err := s.CCrypto.DeriveKey(capsuleMasterKey, []byte(info))

	return err
}

/*
	- files > archive > compress > block sink(encrypt 1mb block) > shard (32|22)54 > a shard to RemotePeer
*/

// todo: so i want to use sqlite and bun orm for rather than bolt. so what svhema will be for my application. design the shcemas with the right relatios. explain everything in the code with commets // for agent
