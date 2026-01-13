package transport

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/engr-sjb/diogel/internal/message"
	"github.com/engr-sjb/diogel/internal/protocol"
	"github.com/google/uuid"
)

const (
	chunkSize = 256 * 1024
)

var (
	ErrChunkSizeExceeded     = errors.New("chunk size exceeded")
	ErrUnexpectedMessageType = errors.New("unexpected message type")
)

type remotePeerConn struct {
	id           uuid.UUID
	conn         net.Conn
	publicKeyStr customcrypto.PublicKeyStr
	publicKey    customcrypto.PublicKeyBytes
	protocol     protocol.Protocol

	writeMu     sync.Mutex
	writeFrame  protocol.Frame
	lastWriteOp atomic.Int64 // lastWriteOp holds the time when a last operation occurred.

	readMu     sync.Mutex
	readFrame  protocol.Frame
	lastReadOp atomic.Int64 // lastReadOp holds the time when a last operation occurred.

	// isBeingRead, isBeingWritten bool
}

// _ is compiler check to prevent silent errors when either RemotePeerConn or remotePeer don't hold same methods.
var _ RemotePeerConn = (*remotePeerConn)(nil)

func NewRemotePeer(
	publicKey customcrypto.PublicKeyBytes,
	conn net.Conn,
	protocol protocol.Protocol) *remotePeerConn {
	// NOTICE IMPORTANT: In order not to do an allocation and then copy just to get a string via hex.EncodeToString(publicKey) or string(publicKey) which is a performance overhead I don't want in this section. So we are using unsafe.String to get the pointer of the first element and then its length. I am doing this cause I know for a fact that there is no reason for the public bytes array or slice to be changed.
	// NOTICE: The RISK 1: If the a byte or bytes of the underlying array or slice is changed, the string will be mutated. Which normal strings in Go don't do; they are immutable.
	// NOTICE: The RISK 2: If the publicKey byte is a byte slice ([]byte) and we use append() on it for whatever reason, the underlying array (unsafe.SliceData(publicKey)) pointers to will not point to the same pointer as before. We will have a dangling pointer. Meaning it will contain garbage data.
	publicKeyStr := unsafe.String(unsafe.SliceData(publicKey), len(publicKey))

	return &remotePeerConn{
		id:           uuid.New(),
		conn:         conn,
		publicKeyStr: customcrypto.PublicKeyStr(publicKeyStr),
		publicKey:    publicKey,
		protocol:     protocol,
	}
}

func (pr *remotePeerConn) Write(p []byte) (n int, err error) {
	pr.writeMu.Lock()
	defer pr.writeMu.Unlock()
	return pr.write(p)
}

func (pr *remotePeerConn) Read(p []byte) (n int, err error) {
	pr.readMu.Lock()
	defer pr.readMu.Unlock()

	return pr.read(p)
}

func (pr *remotePeerConn) Send(msg message.Msg) error {
	pr.writeMu.Lock()
	defer pr.writeMu.Unlock()
	return pr.send(msg)
}

func (pr *remotePeerConn) SendChunk(msg message.Msg, data []byte) (int, error) {
	pr.writeMu.Lock()
	defer pr.writeMu.Unlock()

	if len(data) > chunkSize {
		return 0, ErrChunkSizeExceeded
	}

	if err := pr.send(msg); err != nil {
		return 0, err
	}

	return pr.write(data)
}

func (pr *remotePeerConn) ReceiveChunk(buf []byte) (message.Msg, int, error) {
	pr.readMu.Lock()
	defer pr.readMu.Unlock()

	if err := pr.protocol.ReadFrame(pr, &pr.readFrame); err != nil {
		return nil, 0, err
	}

	var size uint64 = 0

	switch m := pr.readFrame.Payload.Msg.(type) {
	case message.CapsuleStreamChuck:
		size = m.Size

	default:
		return nil, 0, ErrUnexpectedMessageType
	}

	if size > chunkSize || int(size) > len(buf) {
		return nil, 0, ErrChunkSizeExceeded
	}

	n, err := io.ReadFull(pr.conn, buf[:size])
	if err == nil {
		pr.lastReadOp.Store(time.Now().UnixNano())
	}
	return pr.readFrame.Payload.Msg, n, err
}

func (pr *remotePeerConn) write(p []byte) (int, error) {
	n, err := pr.conn.Write(p)
	if err == nil {
		pr.lastWriteOp.Store(time.Now().UnixNano())
	}

	return n, err
}

func (pr *remotePeerConn) send(msg message.Msg) error {
	pr.writeFrame.Payload.Msg = msg
	pr.writeFrame.Version = pr.protocol.Version()

	return pr.protocol.WriteFrame(pr, &pr.writeFrame)
}

func (pr *remotePeerConn) read(p []byte) (int, error) {
	n, err := pr.conn.Read(p)
	if err == nil {
		pr.lastReadOp.Store(time.Now().UnixNano())
	}

	return n, err
}

func (pr *remotePeerConn) ID() uuid.UUID {
	return pr.id
}

func (pr *remotePeerConn) PublicKeyStr() customcrypto.PublicKeyStr {
	return pr.publicKeyStr
}

func (pr *remotePeerConn) PublicKey() customcrypto.PublicKeyBytes {
	return pr.publicKey
}

func (pr *remotePeerConn) IsStale(threshold time.Duration) bool {
	writeNano := pr.lastWriteOp.Load()
	readNano := pr.lastReadOp.Load()

	if writeNano == 0 && readNano == 0 {
		return true
	}

	mostRecentNano := writeNano
	if readNano > writeNano {
		mostRecentNano = readNano
	}

	return time.Since(time.Unix(0, mostRecentNano)) > threshold
}

func (pr *remotePeerConn) Close() error {
	return pr.conn.Close()
}
