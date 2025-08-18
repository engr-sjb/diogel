package tcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"

	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/engr-sjb/diogel/internal/protocol"
	"github.com/engr-sjb/diogel/internal/transport"
)

const (
	defaultMsgBufSize    = 4 * 1024  //4 KiB
	defaultStreamBufSize = 32 * 1024 //32 KiB
)

type TCPTransportConfig struct {
	// NOTICE IMPORTANT: When you add a field, ALWAYS check if it is it's default value in its contractor func.
	Ctx        context.Context
	ShutdownWG *sync.WaitGroup
	PrivateKey []byte // todo: not sure about if this should be allowed here. it could get leaked over the wire.... because we are not decrypting anything here???
	PublicKey  []byte
	// ServerKeyPath  string
	// ServerCertPath string
	// ClientKeyPath  string
	// ClientCertPath string
	Logger         *slog.Logger
	Protocol       protocol.Protocol
	Addr           string
	BootstrapPeers []string //todo: reconsider sending bootstraps here. as i think calling in peer is better.
	DialTimeout    time.Duration
	OnConnect      transport.OnConnect
	OnDisconnect   transport.OnDisconnect
	OnMessage      transport.OnMessage
}

type tcpTransport struct {
	*TCPTransportConfig
	wg *sync.WaitGroup
	ln net.Listener
}

var _ transport.Transport = (*tcpTransport)(nil)

func NewTCPTransport(cfg *TCPTransportConfig) *tcpTransport {
	//todo: return an error
	// NOTICE IMPORTANT: check if all fields on cfg are not their default value before use.
	switch {
	case cfg == nil:
		log.Fatalln("TCPTransportConfig cannot be nil")
	case cfg.Ctx == nil:
		log.Fatalln("Context cannot be nil")
	case cfg.ShutdownWG == nil:
		log.Fatalln("ShutdownWG cannot be nil")
	case cfg.PrivateKey == nil:
		log.Fatalln("PrivateKey cannot be nil")
	case cfg.PublicKey == nil:
		log.Fatalln("PublicKey cannot be nil")
	// case cfg.ServerKeyPath == "":
	// 	log.Fatalln("ServerKeyPath cannot be empty")
	// case cfg.ServerCertPath == "":
	// 	log.Fatalln("ServerCertPath cannot be empty")
	// case cfg.ClientKeyPath == "":
	// 	log.Fatalln("ClientKeyPath cannot be empty")
	// case cfg.ClientCertPath == "":
	// 	log.Fatalln("ClientCertPath cannot be empty")
	case cfg.Logger == nil:
		log.Fatalln("Logger cannot be nil")
	case cfg.Protocol == nil:
		log.Fatalln("Protocol cannot be nil")
	case cfg.Addr == "":
		log.Fatalln("Addr cannot be empty")
	case len(cfg.BootstrapPeers) <= 0:
		log.Fatalln("BootstrapPeers cannot be empty") // todo: not sure if we should pass bootstrap to transport. My just have method on transport that takes in a bootstrap does on connect.
	case cfg.DialTimeout == time.Duration(0):
		log.Fatalln("DialTimeout cannot be zero")
	case cfg.OnConnect == nil:
		log.Fatalln("OnConnect cannot be nil")
	case cfg.OnDisconnect == nil:
		log.Fatalln("OnDisconnect cannot be nil")
	case cfg.OnMessage == nil:
		log.Fatalln("OnMessage cannot be nil")
	}

	t := &tcpTransport{
		TCPTransportConfig: cfg,
		wg:                 &sync.WaitGroup{},
	}

	t.listenAndAccept()
	t.connectBootstrapPeers()

	return t
}

/* As Server Methods Start*/
func (t *tcpTransport) listenAndAccept() {

	// serverKeyPath := ""
	// serverCertPath := ""

	// clientKeyPath := ""
	// clientCertPath := ""

	// serverCert, _ := tls.LoadX509KeyPair(
	// 	serverCertPath,
	// 	serverKeyPath,
	// )

	t.ShutdownWG.Add(1)
	go func() {
		defer t.ShutdownWG.Done()
		var err error

		t.ln, err = net.Listen("tcp", t.Addr)
		if err != nil {
			log.Fatalf(
				"could not listen on %s: %v",
				t.ln.Addr(),
				err,
			)
		}
		defer t.ln.Close()

		// log.Printf(
		// 	"[TCPTransport: %s]: is listening...\n",
		// 	t.ln.Addr(),
		// )

		t.acceptLoop()
	}()
}

func (t *tcpTransport) acceptLoop() {
	// todo: more needed here as the continue and return are confusing me.
	for {
		select {
		case <-t.Ctx.Done():
			log.Printf(
				"[TCPTransport: %s]: is shutting down. Waiting for inflight conns...\n",
				t.ln.Addr(),
			)

			t.wg.Wait()
			return

		default:
			conn, err := t.ln.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					log.Printf(
						"[TCPTransport: %s]: listener closed. shutting down...\n",
						t.ln.Addr(),
					)
					continue
				}

				log.Println(err)
				continue
			}

			remotePublicKey, err := t.Protocol.DoServerHandshake(
				conn,
				t.PublicKey,
			)

			if err != nil {
				log.Printf(
					"[TCPTransport: %s]: [remote peer %s]; could not perform handshake. dropping conn\n",
					t.ln.Addr(),
					conn.RemoteAddr().String(),
				)
				conn.Close()
				return
			}

			peer, err := t.newRemotePeer(
				// ports.PublicKey(remotePublicKeyStr),
				remotePublicKey,
				conn,
			)
			if err != nil {
				log.Println(
					"dropping conn",
					err,
				)
				conn.Close()
				t.OnDisconnect(peer.PublicKeyStr())

				continue
			}

			log.Printf(
				"[TCPTransport: %s]: [remotePeer %s]; connected...\n",
				t.ln.Addr(),
				peer.PublicKeyStr()[:6],
			)

			t.handleRemotePeerConn(peer)
		}
	}
}

func (t *tcpTransport) handleRemotePeerConn(remotePeerConn transport.RemotePeerConn) {

	t.wg.Add(1)
	go func(remotePeerConn transport.RemotePeerConn) {
		defer t.wg.Done()
		defer remotePeerConn.Close()

		// remotePeerConn.

		t.Logger.With(
			slog.String(
				"remotePeerPublicKey",
				string(remotePeerConn.PublicKeyStr()),
			),
		)

		t.OnConnect(remotePeerConn)
		// todo: ideal; since the peer is being add to peer orchestrator, cant we read the message from there??? think about it. we can use only one "callback"(onConnect()) which on connect to add the peer and send peer to the onMessage within the peer orchestrator. but we might leak protocol frame reading into the orchestrator.

		for {
			select {
			case <-t.Ctx.Done():
				log.Printf(
					"[TCPTransport: %s]: [remote peer %s]; conn closed. shutting down...\n",
					t.ln.Addr(),
					remotePeerConn.PublicKey(),
				)
				return

			default:
				readFrame := new(protocol.Frame)
				err := t.Protocol.ReadFrame(remotePeerConn, readFrame)
				if err != nil {
					if errors.Is(err, io.EOF) {
						log.Printf(
							"[TCPTransport: %s]: [remote peer %s]; readFrame err: %v. dropping conn and removing it.\n",
							t.ln.Addr(),
							remotePeerConn.PublicKey(),
							err,
						)

						t.OnDisconnect(remotePeerConn.PublicKeyStr())
						return
					}

					log.Printf(
						"[TCPTransport: %s]: [remote peer %s]; readFrame err: %v going back to reading\n",
						t.ln.Addr(),
						remotePeerConn.PublicKey(),
						err,
					)
					return
				}

				t.OnMessage(remotePeerConn, readFrame.Payload.Msg)
			}
		}
	}(remotePeerConn)
}

/* As Server Methods End */

/* As Client methods Start */

func (t *tcpTransport) ConnectToPeer(addr string) (transport.RemotePeerConn, error) {
	return t.connect(addr)
}

func (t *tcpTransport) dial(addr string) (net.Conn, error) {
	return net.Dial("tcp", addr)
}

func (t *tcpTransport) connectBootstrapPeers() {
	defer t.ShutdownWG.Add(1)

	go func() {
		defer t.ShutdownWG.Done()

		wg := sync.WaitGroup{}

		half := math.Ceil(
			max(
				(float64(len(t.BootstrapPeers)) / 2.00),
				1.00,
			),
		)
		log.Println(">>>>>>>>>>>>>>>", half)
		// half := max(len(t.BootstrapPeers)/2, 1)

		for i := range t.BootstrapPeers {
			if len(t.BootstrapPeers) > 1 {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()

					remotePeer, err := t.connect(
						t.BootstrapPeers[i],
					)
					log.Printf(
						"{dialer: %s} ->%s",
						t.BootstrapPeers[i],
						// remotePeer.PublicKey(),
						t.Addr,
					)
					log.Println("calling in one work")

					if err != nil {
						log.Printf(
							"{dialer: %s} failed to add new peer addr%s: %v\n",
							// remotePeer.PublicKey(),
							t.Addr,
							t.BootstrapPeers[i],
							err,
						)
						return
						// continue
					}

					t.OnConnect(remotePeer)
				}(i)

				return
			}
		}

		wg.Wait()
	}()
}

func (t *tcpTransport) connect(addr string) (transport.RemotePeerConn, error) {
	var (
		retryMax = 5

		retryDelay = 500 * time.Millisecond
		// retryExponent = 1.0

		conn net.Conn
		err  error
	)

	for i := range retryMax {
		conn, err = t.dial(addr)
		if err != nil {
			log.Printf(
				"failed to dial peer addr %s, count=%d:retrying in %d secs; %v\n",
				addr,
				i+1,
				retryDelay,
				err,
			)

			time.Sleep(retryDelay)

			// retryDelay = time.Duration(
			// 	float64(retryDelay) * math.Pow(2, retryExponent),
			// )
			// retryDelay = retryDelay * time.Duration(1<<i)
			retryDelay *= 2

			if i == (retryMax - 1) {
				return nil, fmt.Errorf("couldn’t dial peer addr %s after %d attempts: %w",
					addr,
					retryMax,
					err,
				)
			}

			continue
		}

		break
	}

	publicKey, err := t.Protocol.DoClientHandshake(conn, t.PublicKey)
	if err != nil {
		return nil, errors.New("remote peer failed failed client handshake")
	}

	remotePeer, err := t.newRemotePeer(
		publicKey,
		conn,
	)
	if err != nil {
		return nil, err
	}
	return remotePeer, nil
}

/* As Client methods End */

func (t *tcpTransport) Close() error {
	return t.ln.Close()
}

func (t *tcpTransport) newRemotePeer(publicKey []byte, conn net.Conn) (transport.RemotePeerConn, error) {
	switch {
	case publicKey == nil:
		return nil, errors.New("publicKey can't be nil")
	case conn == nil:
		return nil, errors.New("conn can't be nil")
	}

	rp := transport.NewRemotePeer(publicKey, conn, t.Protocol)

	return rp, nil
}
