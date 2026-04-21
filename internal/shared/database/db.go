package database

const (
	BucketUser = "user"

	BucketCapsules             = "capsules"
	BucketCapsulesActiveShards = "capsules:active_shards"
	BucketCapsuleManifests     = "capsules:manifests"
	BucketCapsulesRecovery     = "capsules:recovery"

	BucketGuardians  = "guardians"
	BucketKeyShares  = "keyshares"
	BucketHeartbeats = "heartbeats"
	BucketPeers      = "peers"
)

//todo: add a struct for every bucket group type or feature to limit the access of them in different feature slices.

// Todo: i might have to let let multiple coll have one bucket ame. not sure. or have sub buckets, not sure.

type Collection uint16

const (
	CollUser Collection = iota

	CollCapsules
	CollCapsulesActiveShards
	CollCapsuleManifests
	CollCapsulesRecovery

	CollGuardians
	CollKeyShares
	CollHeartbeats
	CollPeers
)

func (c Collection) BucketName() string {
	switch c {
	case CollUser:
		return BucketUser

	case CollCapsules:
		return BucketCapsules
	case CollCapsulesActiveShards:
		return BucketCapsulesActiveShards
	case CollCapsuleManifests:
		return BucketCapsuleManifests
	case CollCapsulesRecovery:
		return BucketCapsulesRecovery

	case CollGuardians:
		return BucketGuardians
	case CollKeyShares:
		return BucketKeyShares
	case CollHeartbeats:
		return BucketHeartbeats
	case CollPeers:
		return BucketPeers
	default:
		return ""
	}
}
