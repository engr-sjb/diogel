package database

const (
	BucketCapsules         = "capsules"
	BucketCapsulesActive   = "capsules:active"
	BucketCapsulesRecovery = "capsules:recovery"

	BucketGuardians  = "guardians"
	BucketKeyShares  = "keyshares"
	BucketHeartbeats = "heartbeats"
	BucketPeers      = "peers"
)

///todo: add a struct for every bucket group type or feature to limit the access of them in different feature slices.

type Collection uint16

const (
	CollCapsules Collection = iota
	CollCapsulesActive
	CollCapsulesRecovery
	CollGuardians
	CollKeyShares
	CollHeartbeats
	CollPeers
)

func (c Collection) BucketName() string {
	switch c {
	case CollCapsules:
		return BucketCapsules
	case CollCapsulesActive:
		return BucketCapsulesActive
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
