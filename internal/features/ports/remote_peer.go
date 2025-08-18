package ports

import (
	"io"
	"time"

	"github.com/engr-sjb/diogel/internal/message"
)

type PublicKey string // todo: move this to transport i think. no other part needs it here as nno one uses string public key. but wait, capsule will need a slice of pub key.

type RemotePeer interface {
	io.ReadWriter
	PublicKeyStr() PublicKey
	PublicKey() []byte
	IsBeingRW() (isBeingRead bool, isBeingWritten bool)
	IsStale(threshold time.Duration) bool
	Send(msg message.Msg) error
}
