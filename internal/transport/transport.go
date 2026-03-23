package transport

import (
	"github.com/engr-sjb/diogel/internal/message"
	"github.com/google/uuid"
)

type OnConnect func(RemotePeerConn) error
type OnDisconnect func(remotePeerID uuid.UUID) error
type OnMessage func(remotePeer RemotePeer, msg message.Msg) //Todo: might have to move this if i don't want import cycle

type TransportServer interface {
}

type Transport interface {
	ConnectToPeer(addrs string) (RemotePeerConn, error)
	// Close closes the transport listener.
	Close() error
}
