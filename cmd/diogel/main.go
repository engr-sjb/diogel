package main

import (
	"log"

	"github.com/engr-sjb/diogel/cmd/diogel/peer"
)

func main() {
	log.SetFlags(log.Llongfile | log.Ltime)

	const (
		peer1 = ":3000"
		peer2 = ":3001"
		peer3 = ":3002"
		peer4 = ":3003"
		peer5 = ":3004"
		peer6 = ":3005"
	)

	newApp(peer1, peer2, peer3)

	// peers := []string{peer1, peer2, peer3, peer4, peer5, peer6}
	// for _, p := range peers[1:] {
	// 	go newApp(p)
	// }

	// newApp(peers[0], peers[1:]...)

	// select {}
}

func newApp(addr string, bootstrapPeers ...string) {
	// log.SetPrefix(
	// 	fmt.Sprint(
	// 		"peer", addr, ": ",
	// 	),
	// )
	p := peer.NewPeer(
		&peer.PeerConfig{
			Addr:           addr,
			BootstrapPeers: bootstrapPeers,
			UserBucketName: "user",
		},
	)
	p.Run()
	// if addr == ":3000" {
	// 	// p.AddFile("test.txt")
	// }

}
