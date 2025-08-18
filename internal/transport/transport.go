package transport

import (
	"io"

	"github.com/engr-sjb/diogel/internal/features/ports"
	"github.com/engr-sjb/diogel/internal/message"
)

type RemotePeerConn interface {
	io.Closer
	ports.RemotePeer
}

type OnConnect func(RemotePeerConn) error
type OnDisconnect func(publicKeyStr ports.PublicKey) error
type OnMessage func(remotePeer ports.RemotePeer, msg message.Msg)

type TransportServer interface {
}

type Transport interface {
	ConnectToPeer(addrs string) (RemotePeerConn, error)
	// Close closes the transport listener.
	Close() error
}
