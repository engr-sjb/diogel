package peer

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/engr-sjb/diogel/internal/customcrypto"
	"github.com/engr-sjb/diogel/internal/features/capsule"
	"github.com/engr-sjb/diogel/internal/features/ports"
	"github.com/engr-sjb/diogel/internal/features/user"
	"github.com/engr-sjb/diogel/internal/protocol"
	"github.com/engr-sjb/diogel/internal/serialize"
	"github.com/engr-sjb/diogel/internal/storage"
	"github.com/engr-sjb/diogel/internal/transport"
	"github.com/engr-sjb/diogel/internal/transport/tcp"
	bolt "go.etcd.io/bbolt"
)

type features struct {
	*user.User
	*capsule.Capsule
}

type PeerConfig struct {
	// NOTICE IMPORTANT: When you add a field, ALWAYS check if it is it's default value in its contractor func.

	Addr           string
	UserBucketName string
	BootstrapPeers []string
}

type peer struct {
	*PeerConfig
	privateKey []byte
	publicKey  []byte
	shutdownWG *sync.WaitGroup
	db         *bolt.DB
	serialize  serialize.Serializer
	protocol   protocol.Protocol
	cCrypto    *customcrypto.CCrypto
	transport  transport.Transport
	features   *features

	connectedRemotePeersMu sync.RWMutex
	connectedRemotePeers   map[ports.PublicKey]transport.RemotePeerConn
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
	case cfg.UserBucketName == "":
		log.Fatalln("BucketName field in PeerConfig cannot be empty")
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
	}
}

func (p *peer) Run() {
	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	defer cancel()

	p.prepDeps(ctx)
	// p.serialize.Register(
	// 	msg.Msgs...,
	// )
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
		"./.diogel/%s",
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
}

// prepFeatures prepares and initializes and configures the peer's features.
func (p *peer) prepFeatures(ctx context.Context) {
	userDBStore := user.NewDBStore(
		&user.DBStoreConfig{
			DB:                    p.db,
			UserBucketName:        p.UserBucketName,
			UserSettingBucketName: "settings",
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

	p.privateKey, p.publicKey = p.features.User.Service.GetKeyPair()

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

