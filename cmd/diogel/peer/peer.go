/*
	Copyright (c) 2025 Stephen Jersuit Benyah
	Licensed under the Repo-Only Non-Commercial & No-Derivatives License with Anti-Training Clause (RONCND-AT) v1.0.
	See LICENSE and CONTRIBUTION_LICENSE_AGREEMENT.md in repository root.
	Prohibited: copying, reuse, redistribution, or use as training data for machine learning/AI.
*/

package peer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/engr-sjb/diogel/internal/archive"
	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/engr-sjb/diogel/internal/dataredundancy"
	"github.com/engr-sjb/diogel/internal/features/capsule"
	"github.com/engr-sjb/diogel/internal/features/ports"
	"github.com/engr-sjb/diogel/internal/features/user"
	"github.com/engr-sjb/diogel/internal/message"
	"github.com/engr-sjb/diogel/internal/protocol"
	"github.com/engr-sjb/diogel/internal/serialize"
	"github.com/engr-sjb/diogel/internal/storage"
	"github.com/engr-sjb/diogel/internal/transport"
	"github.com/engr-sjb/diogel/internal/transport/tcp"
	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

type features struct {
	*user.User
	*capsule.Capsule
}

type PeerConfig struct {
	// NOTICE IMPORTANT: When you add a field, ALWAYS check if it is it's default value in its contractor func.

	Addr                    string
	appDir                  string
	BootstrapPeers          []string
	MinConnectedRemotePeers uint32
}

type peer struct {
	*PeerConfig
	peerID     uuid.UUID
	privateKey []byte
	publicKey  []byte
	shutdownWG *sync.WaitGroup
	db         *bolt.DB
	serialize  serialize.Serializer
	protocol   protocol.Protocol
	cCrypto    customcrypto.CCrypto
	archive    archive.Archiver

	transport transport.Transport
	features  *features

	connectedRemotePeersMu sync.RWMutex
	connectedRemotePeers   map[uuid.UUID]transport.RemotePeerConn

	// connectedRemotePeersTest struct {
	// 	conns transport.RemotePeerConns
	// }

	// connectedBootstrapPeersMu     sync.RWMutex // todo: create a method on peer to connect bootstrap peers which then calls a method on transport to connect which returns a remoteBootstrapPeer. maybe have a different strap for them or just use one remote peer struct and have more methods on it for bootstrap activities. not sure yet.
	// connectedBootstrapRemotePeers map[ports.PublicKey]transport.RemotePeerConn
}

func NewPeer(cfg *PeerConfig) *peer {
	// NOTICE IMPORTANT: check if all fields on cfg are not their default value before use.
	switch {
	case cfg == nil:
		log.Fatalln("PeerConfig cannot be nil")
	case cfg.Addr == "":
		log.Fatalln("Addr field in PeerConfig cannot be empty")
	case len(cfg.BootstrapPeers) == 0:
		log.Fatalln("BootstrapPeers field in PeerConfig cannot be empty")
	case cfg.MinConnectedRemotePeers == 0:
		cfg.MinConnectedRemotePeers = 50
	}

	// NOTICE IMPORTANT: make sure you are initializing the fields on the returned struct that need to be initialized.
	return &peer{
		PeerConfig: cfg,
		shutdownWG: &sync.WaitGroup{},
		features: &features{
			// NOTICE IMPORTANT:
			User:    &user.User{},
			Capsule: &capsule.Capsule{},
		},
		connectedRemotePeers: make(map[uuid.UUID]transport.RemotePeerConn),
	}
}

func (p *peer) Run() {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	defer cancel()

	p.prepDeps(ctx)
	p.serialize.Register(
		message.Msgs...,
	)
	//todo: fix this register error as duplicates

	p.prepFeatures(ctx)

}

// prepDeps prepares and initializes the peer's dependencies need by the various components.
func (p *peer) prepDeps(ctx context.Context) {
	// home, err := os.UserHomeDir()
	// if err != nil {
	// 	log.Fatalf("Error getting user home directory: %v", err)
	// }

	directory := fmt.Sprintf(
		"./.diogel/%s", //todo:  this should come from the config.
		p.Addr,
	)

	err := os.MkdirAll(
		directory,
		0700,
	)
	if err != nil {
		log.Fatal("Error creating .diogel directory")
	}

	p.db = storage.NewBBolt(directory, p.logger)
	p.serialize = serialize.New()
	p.protocol = protocol.NewProtocol(p.serialize)
	p.cCrypto = customcrypto.NewCCrypto()
	p.archive = archive.NewArchive() //todo: i think the depends too should take in the shutdown waitGroup too. not sure yet. think about as we already inject into the features.
}

// prepFeatures prepares and initializes and configures the peer's features.
func (p *peer) prepFeatures(ctx context.Context) {
	userDBStore := user.NewDBStore(
		&user.DBStoreConfig{
			DB:                    p.db,
			UserSettingBucketName: "settings", // todo: sort these bucket names properly.
		},
	)

	p.features.User.Service = user.NewService(
		&user.ServiceConfig{
			Ctx:     ctx,
			DBStore: userDBStore,
			CCrypto: p.cCrypto,
		},
	)

	pwd := "fake_password" // todo: should come from ui.
	if err := p.features.User.Service.InitIdentity(pwd); err != nil {
		log.Fatalf("failed to init user identity: %v", err)
	}

	p.peerID, p.privateKey, p.publicKey = p.features.User.Service.GetIdentity()

	onMessage := p.makeOnMessageHandler(ctx)

	p.transport = tcp.NewTCPTransport(
		&tcp.TCPTransportConfig{
			Ctx:            ctx,
			ShutdownWG:     p.shutdownWG,
			PrivateKey:     p.privateKey,
			PublicKey:      p.publicKey,
			Addr:           p.Addr,
			BootstrapPeers: p.BootstrapPeers,
			Logger:         p.logger,
			Protocol:       p.protocol,
			DialTimeout:    time.Second * 2, // todo: reevaluate
			OnConnect:      p.onConnect,
			OnDisconnect:   p.onDisconnect,
			OnMessage:      onMessage,
		},
	)

	// Capsule Feature
	capsuleDBStore := capsule.NewDBStore(
		&capsule.DBStoreConfig{
			DB: p.db,
		},
	)

	capsuleObjectStore := capsule.NewObjectStore(
		&capsule.FileStoreConfig{
			RootDir: p.appDir,
		},
	)

	p.features.Capsule.Service = capsule.NewService(
		&capsule.ServiceConfig{
			Ctx:      ctx,
			Shutdown: p.shutdownWG,
			Defaults: &capsule.Defaults{
				MinNumOfGuardians: 3,
				//todo: we need the max. we might have to pull this from a subscription plan inn future. not sure yet.
			},
			PeerID:              p.peerID,
			PrivateKey:          p.privateKey,
			PublicKey:           p.publicKey,
			Archive:             p.archive,
			CCrypto:             p.cCrypto,
			Serialize:           p.serialize,
			DBStore:             capsuleDBStore,
			FileStore:           capsuleObjectStore,
			NewErasureCoderFunc: dataredundancy.NewReedSolomonCoder,
			//todo: should take a callback function that searches thru connected peers and populate the
		},
	)
}

func (p *peer) makeOnMessageHandler(ctx context.Context) transport.OnMessage {

	return func(remotePeer transport.RemotePeer, msg message.Msg) {
		err := p.onMessage(ctx, remotePeer, msg)
		if pErr, isPErr := errors.AsType[*peererrors.PeerError](err); isPErr {
			switch scope := pErr.Scope(); scope {
			case peererrors.ScopeRemotePeer:
				errMsg := &message.ErrorMessage{
					Code:    pErr.Code(),
					Message: pErr.Error(),
				}
				_, err := remotePeer.Send(errMsg, nil)
				if err != nil {
					log.Printf("failed to send error message to remote peer %s: %v", remotePeer.ID(), err)
				}
				fmt.Printf("remote peer %s error: %s", remotePeer.ID(), pErr.Error())

			case peererrors.ScopeInternalPeer:
				fmt.Printf("internal peer error: %s", pErr.Error())

			case peererrors.ScopeLocalPeer:
				fmt.Printf("local peer error: %s", pErr.Error())

			default:
				log.Printf("unhandled peer error code %d: %s", scope, pErr.Error())
			}
		}

		log.Println(err)

	}
}

func (p *peer) onMessage(ctx context.Context, remotePeer transport.RemotePeer, msg message.Msg) error {
	msgCtx, cancel := context.WithTimeout(
		ctx,
		(time.Second * 2), // todo: reconsider this time
	)
	defer cancel()

	switch newMsg := msg.(type) {
	// Capsule Feature
	case message.CapsuleIncomingStream:
		err := p.features.Capsule.Service.ReceiveCapsuleStream(
			msgCtx,
			remotePeer,
			&newMsg,
		)

		if err != nil {
			return err
		}

		// Todo:  Add proper data
		if err := p.features.Heartbeat.Service.Add(); err != nil {
			return err
		}

		log.Println("incoming capsule stream")

	case message.ContinueCapsuleStream:
		/*
			todo - call the service to act on message if thats whats needed.
			todo or
			todo - call the ui to display something if the user need to confirm an action before it takes place.
		*/
		log.Println("incoming Re capsule stream")

	case message.CapsuleReStream:
		/*
			todo - call the service to act on message if thats whats needed.
			todo or
			todo - call the ui to display something if the user need to confirm an action before it takes place.
		*/
		log.Println("incoming Re capsule stream")

	// Heartbeat Feature
	case message.HeartbeatCheck:
		log.Println("incoming HeartbeatCheck")

	default:
		log.Println(
			"unknown msg in router",
		)
	}

	return nil
}
	}
}

// onConnect is passed to the transport to be used to register newly connected
// remote peers to this peer's internal memory map.
func (p *peer) onConnect(newRemotePeerConn transport.RemotePeerConn) error {
	staleThreshold := 35 * time.Minute // todo: might have to make this a global variable in here or on peer. so we can use go eviction of connect that are stale after this time. use a ticker in ggo routine to check every staleThreshold.

	p.connectedRemotePeersMu.RLock()
	oldRemotePeerConn, isFound := p.connectedRemotePeers[newRemotePeerConn.ID()]
	p.connectedRemotePeersMu.RUnlock()

	if isFound {
		isStale := oldRemotePeerConn.IsStale(staleThreshold)
		if isStale {
			p.connectedRemotePeersMu.Lock()
			p.connectedRemotePeers[newRemotePeerConn.ID()] = newRemotePeerConn
			p.connectedRemotePeersMu.Unlock()
			return nil
		}

		return nil
	}

	p.connectedRemotePeersMu.Lock()
	p.connectedRemotePeers[newRemotePeerConn.ID()] = newRemotePeerConn
	p.connectedRemotePeersMu.Unlock()

	return nil
}

func (p *peer) onDisconnect(remotePeerID uuid.UUID) error {
	defer p.connectedRemotePeersMu.Unlock()

	p.connectedRemotePeersMu.Lock()
	delete(p.connectedRemotePeers, remotePeerID)

	return nil
}

func (p *peer) findRemotePeersBy(addrs []string) ([]transport.RemotePeer, error) {
	rps := make([]transport.RemotePeer, len(addrs))
	// ch := make(chan transport.RemotePeerConn)
	wg := &sync.WaitGroup{}

	// for _ = range addrs {
	// p.connectedPeersMu.RLock()
	//remotePeers[i] := p.connectedRemotePeers[publicKeyStr]  // todo: might delete as we don't have public keys. so we just assume we don't have these remotePeerConn already. i need to fix this. need to do that with discovery or something.
	// p.connectedBootstrapPeersMu.RUnlock()
	// }

	// remotePeers, err := p.transport.Connect(addrs)
	for i := range addrs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			remotePeerConn, err := p.transport.ConnectToPeer(
				addrs[i],
			)
			if err != nil {
				log.Println(err)
				return
			}

			if err := p.onConnect(remotePeerConn); err != nil {
				log.Println(err)
				return
			}
			rps[i] = remotePeerConn
		}(i)
	}

	wg.Wait()

	return rps, nil
}

func (p *peer) closeConnectedPeers() error {
	defer p.connectedRemotePeersMu.RUnlock()

	// var err error //todo: gather errors and return them but try and close them.
	p.connectedRemotePeersMu.RLock()
	for key := range p.connectedRemotePeers {
		// I am looping this way so i don't copy the peer in the loop.
		if err := p.connectedRemotePeers[key].Close(); err != nil {
			return err
		}
	}
	return nil
}

// ////////////////////////////////
// Methods for UI/CLI use or
func (p *peer) Create(ctx context.Context, letterContent string, filePaths []string, guardiansAddrs []string, silencePeriod time.Duration) error {
	//todo: we need to derived from our shutdown context or the request context. not sure

	remotePeers, err := p.findRemotePeersBy(guardiansAddrs)
	if err != nil {
		return err
	}

	content := strings.NewReader(letterContent)

	cc := &capsule.CreateCapsuleDTO{
		RemotePeerGuardians: remotePeers,
		Letter: &ports.FileMem{
			Name:    capsule.LetterName,
			Content: io.NopCloser(content),
			Mode:    0644,
			ModTime: time.Now(),
			Size:    content.Size(),
		},
		FilePaths:     filePaths,
		SilencePeriod: silencePeriod,
	}

	return p.features.Capsule.Service.CreateAndSendCapsule(ctx, cc)
}
