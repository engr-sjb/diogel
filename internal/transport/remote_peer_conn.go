package transport

import (
	"encoding/hex"
	"net"
	"sync"
	"time"

	"github.com/engr-sjb/diogel/internal/features/ports"
	"github.com/engr-sjb/diogel/internal/message"
	"github.com/engr-sjb/diogel/internal/protocol"
)

type remotePeerConn struct {
	conn         net.Conn
	publicKeyStr ports.PublicKey
	publicKey    []byte
	protocol     protocol.Protocol

	mu                          sync.RWMutex
	isBeingRead, isBeingWritten bool
	lastOp                      time.Time // lastOp holds the time when a last operation occurred.
}

// _ is compiler check to prevent silent errors when either RemotePeerConn or remotePeer don't hold same methods.
var _ RemotePeerConn = (*remotePeerConn)(nil)

func NewRemotePeer(publicKey []byte, conn net.Conn, proto protocol.Protocol) *remotePeerConn {
	publicKeyStr := hex.EncodeToString(publicKey) // todo: maybe; use the unsafe package to convert to string for better performance. We wont be mutating the public key byte as it will be the underlying data for the string.

	return &remotePeerConn{
		conn:         conn,
		publicKeyStr: ports.PublicKey(publicKeyStr),
		publicKey:    publicKey,
		protocol:     proto,
		lastOp:       time.Now(),
	}
}

func (pr *remotePeerConn) Read(p []byte) (n int, err error) {
	pr.mu.Lock()
	pr.isBeingRead = !pr.isBeingRead
	n, err = pr.conn.Read(p)
	pr.isBeingRead = !pr.isBeingRead
	pr.mu.Unlock()

	return n, err
}

func (pr *remotePeerConn) Write(p []byte) (n int, err error) {
	pr.mu.Lock()
	pr.isBeingWritten = !pr.isBeingWritten
	n, err = pr.conn.Write(p)
	pr.isBeingWritten = !pr.isBeingWritten
	pr.lastOp = time.Now()
	pr.mu.Unlock()

	return n, err
}

func (pr *remotePeerConn) Send(msg message.Msg) error {
	writeFrame := protocol.Frame{
		Version: pr.protocol.Version(),
		Payload: protocol.Payload{
			Msg: msg,
		},
	}
	return pr.protocol.WriteFrame(pr, &writeFrame)
}

func (pr *remotePeerConn) PublicKeyStr() ports.PublicKey {
	return pr.publicKeyStr
}

func (pr *remotePeerConn) PublicKey() []byte {
	return pr.publicKey
}

func (pr *remotePeerConn) IsBeingRW() (isBeingRead bool, isBeingWritten bool) {
	return pr.isBeingRead, pr.isBeingWritten
}

func (pr *remotePeerConn) IsStale(threshold time.Duration) bool {
	defer pr.mu.RUnlock()

	pr.mu.RLock()
	if pr.lastOp.IsZero() {
		// if time is zero value, we consider it as stale since its not been operated on
		return true
	}

	return time.Since(pr.lastOp) > threshold
}

func (pr *remotePeerConn) Close() error {
	return pr.conn.Close()
}
