/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package capsule

import (
	"time"

	"github.com/google/uuid"
)

type capsule struct {
	OwnerID                  uuid.UUID
	GuardianIDs              []uuid.UUID
	CreatedAt                time.Time
	ReceivedAt               time.Time
	CompletedAt              time.Time
	AreShardsReceived        bool
	IsManifestReceived       bool
	IsKeyMasterShareReceived bool
	IsComplete               bool
}

// Mapper functions - convert between domain and database models
func (c *capsule) toDB(id uuid.UUID) *capsuleDB {
	return &capsuleDB{
		ID:                 id,
		OwnerID:            c.OwnerID,
		CreatedAt:          c.CreatedAt,
		CompletedAt:        c.CompletedAt,
		IsKeyShareReceived: c.IsKeyMasterShareReceived,
		IsComplete:         c.IsComplete,
	}
}

type masterKeyShare struct {
	CapsuleID       uuid.UUID
	Share           []byte
	TotalShares     int
	ThresholdShares int
}

type owner struct {
	ID   uuid.UUID
	Name string
	Type string
	Addr string
}

type guardian struct {
	ID   uuid.UUID
	Name string
	Type string
	Addr string
}

// func (c *Capsule) CreateCapsule(ownerID string, guardianIDs []string, content string) {
// 	c.OwnerID = ownerID
// 	c.GuardianIDs = guardianIDs
// 	c.Content = content
// }
