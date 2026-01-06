package transport

import (
	"io"
	"time"

	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/engr-sjb/diogel/internal/features/ports"
	"github.com/engr-sjb/diogel/internal/message"
)

type RemotePeerConn interface {
	io.Closer
	IsStale(threshold time.Duration) bool
	ports.RemotePeer
}

type OnConnect func(RemotePeerConn) error
type OnDisconnect func(publicKeyStr customcrypto.PublicKeyStr) error
type OnMessage func(remotePeer ports.RemotePeer, msg message.Msg) //Todo: might have to move this if i don't want import cycle

type TransportServer interface {
}

type Transport interface {
	ConnectToPeer(addrs string) (RemotePeerConn, error)
	// Close closes the transport listener.
	Close() error
}
