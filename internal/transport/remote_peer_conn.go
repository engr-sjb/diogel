package transport

import (
	"encoding/hex"
	"net"
	"sync"
	"time"
	"unsafe"

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
	// NOTICE IMPORTANT: In order not to do an allocation and then copy just to get a string via hex.EncodeToString(publicKey) or string(publicKey) which is a performance overhead I don't want in this section. So we are using unsafe.String to get the pointer of the first element and then its length. I am doing this cause I know for a fact that there is no reason for the public bytes array or slice to be changed.
	// NOTICE: The RISK 1: If the a byte or bytes of the underlying array or slice is changed, the string will be mutated. Which normal strings in Go don't do; they are immutable.
	// NOTICE: The RISK 2: If the publicKey byte is a byte slice ([]byte) and we use append() on it for whatever reason, the underlying array (&publicKey[0]) pointers to will not point to the same pointer as before. We will have a dangling pointer. Meaning it will contain garbage data.
	publicKeyStr := unsafe.String(&publicKey[0], len(publicKey))

	return &remotePeerConn{
		conn:         conn,
		publicKeyStr: ports.PublicKey(publicKeyStr),
		publicKey:    publicKey,
		protocol:     protocol,
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
