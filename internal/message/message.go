/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package message

import (
	"time"

	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/google/uuid"
)

type Msg any

// Msgs is a collection of message types used in the peer system. Note that the
// messages can contain both by value or by reference of the same type.
//
/*
eg. var Msgs = [8]any{
	&CapsuleStream{}, // Has a pointer type
	CapsuleStream{}, // Has a value type
	// Both the above types are of the same type but by different values.
	&ReCapsuleStream{},
	ReCapsuleStream{},
	&ContinueCapsuleStream{},
	ContinueCapsuleStream{},
	&HeartbeatCheck{},
	HeartbeatCheck{},
}
*/
//
// IMPORTANT NOTE: ALWAYS ADD ALL AND EVERY MESSAGE YOU CREATE TO THIS ARRAY,
// AND INCREASE THE ARRAY SIZE BY EXACTLY THE NUMBER OF MESSAGES ADDED. THIS IS
// USED TO REGISTER THE MESSAGES WITH THE SERIALIZER
//
// It contains the following message types:
//   - CapsuleStream: Handles initial capsule streaming with various relevant information.
//   - ReCapsuleStream: Manages recapsuling of a capsule.
//   - ContinueCapsuleStream: Controls continuation of interrupted capsule streams.
//   - HeartbeatCheck: Manages heartbeat verification.
var Msgs = []any{ // todo: do another research on gob registration of types. both pointer and value types are consider same. not sure yet
	// notice: ... add note here
	// &CapsuleStream{},
	CapsuleStream{},
	// &ReCapsuleStream{},
	ReCapsuleStream{},
	// &ContinueCapsuleStream{},
	ContinueCapsuleStream{},
	// &DeleteCapsule{},
	DeleteCapsule{},
	// &HeartbeatCheck{},
	HeartbeatCheck{},
	RecoveryCeremony{},
}

type CapsuleStream struct {
	ID uuid.UUID

	/*
	* todo: add the guardians slice here. not sure if its public key
	 */
	// GuardiansPublicKey []ports.PublicKeyRaw
	GuardiansPublicKey []customcrypto.PublicKeyBytes
	GuardiansAddr      []string

	/*
		_ todo: TotalSize will be unnecessary as we will be streaming straight from streamEnc(src,dst, encKey).
		_ i think the best ideas i have are. - prepend every streamEnc with a enc size written as the first byte.
		_ so we can peakBuf it to know the size the incoming enc stream.
	*/

	TotalSize uint64

	// todo:ChuckSize and TotalSize will help us implement [ContinueCapsuleStream] after interruption.
	ChuckSize            uint32
	HeartbeatGracePeriod time.Duration
}

type CapsuleStreamChuck struct {
	IsFinal bool
	ID      uuid.UUID
	Seq     uint32
	Size    uint64
}

type ReCapsuleStream struct {
	ID uuid.UUID
	// GuardiansPublicKey []ports.PublicKeyRaw
	GuardiansPublicKey []customcrypto.PublicKeyBytes
	GuardiansAddr      []string // Todo: Might not be necessary.

	// todo: TotalSize will be unnecessary as we will be streaming straight from streamEnc(src,dst, encKey). i think the best ideas i have are. - prepend every streamEnc with a enc size written as the first byte. so we can peakBuf it to know the size the incoming enc stream.

	TotalSize            uint64
	HeartbeatGracePeriod time.Time
}

type DeleteCapsule struct {
}

type ContinueCapsuleStream struct {
	ID             uuid.UUID
	CapsuleID      uuid.UUID
	OffsetChuckNum uint16
}

type HeartbeatCheck struct {
	ID uuid.UUID
	//todo: i am not too sure, about the capsule ID yet.
	CapsuleID  uuid.UUID
	UserPubKey []byte
}

type RecoveryCeremony struct {
}
