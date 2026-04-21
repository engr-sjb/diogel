package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/engr-sjb/diogel/cmd/diogel/peer"
)

func main() {
	log.SetFlags(log.Llongfile | log.Ltime)

	wg := &sync.WaitGroup{}

	const (
		peer6 = ":3006"
		peer5 = ":3005"
		peer4 = ":3004"
		peer3 = ":3003"
		peer2 = ":3002"
		peer1 = ":3001"
	)

	newApp(wg, peer2)
	time.Sleep(1 * time.Second)
	newApp(wg, peer1, peer2)

	// peers := []string{peer1, peer2, peer3, peer4, peer5, peer6}
	// for _, p := range peers[1:] {
	// 	go newApp(p)
	// }

	// newApp(peers[0], peers[1:]...)

	wg.Wait()
}

func newApp(wg *sync.WaitGroup, addr string, bootstrapPeers ...string) {

	wg.Go(func() {
		// log.SetPrefix(
		// 	fmt.Sprint(
		// 		"peer", addr, ": ",
		// 	),
		// )

		logPrefix := fmt.Sprintf("[PEER:==> %s] ", addr)
		logger := log.New(
			os.Stdout,
			logPrefix,
			log.Llongfile|log.Ltime,
		)

		p := peer.NewPeer(
			&peer.PeerConfig{
				Addr:           addr,
				BootstrapPeers: bootstrapPeers,
			},
		)
		go p.Run()

		if addr == ":3001" {
			time.Sleep(5 * time.Second)
			if err := p.Create(
				context.TODO(),
				"the letter",
				[]string{},
				bootstrapPeers,
				(278 * time.Hour)); err != nil {
				logger.Fatal(err)
			}
		}

		select {}

	})
}
