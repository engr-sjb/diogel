/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package ports

import (
	"io"

	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/engr-sjb/diogel/internal/message"
	"github.com/google/uuid"
)

type RemotePeer interface {
	io.ReadWriter
	PublicKeyStr() customcrypto.PublicKeyStr
	PublicKey() customcrypto.PublicKeyBytes
	ID() uuid.UUID
	Send(msg message.Msg) error
}
