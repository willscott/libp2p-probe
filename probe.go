package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	noise "github.com/libp2p/go-libp2p-noise"

	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	tls "github.com/libp2p/go-libp2p-tls"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
)

func main() {
	args := os.Args

	if len(args) < 2 {
		fmt.Printf("usage: probe <multiaddr|pid>\n")
		return
	}

	key, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 0)
	if err != nil {
		fmt.Errorf("could not make host: %s\n", err)
		return
	}

	h, err := NewHost(key, []multiaddr.Multiaddr{})
	if err != nil {
		fmt.Errorf("could not make host: %s\n", err)
		return
	}
	fmt.Printf("host made...\n")

	ma := args[1]
	var ai *peer.AddrInfo
	parsed, err := multiaddr.NewMultiaddr(ma)
	if err != nil {
		// try to find the multiaddrs.
		pid, err := peer.Decode(ma)
		if err != nil {
			fmt.Printf("could not parse peer: %s\n", err)
			return
		}
		ai, err = findPeer(h, pid)
		if err != nil {
			fmt.Printf("could not find peer: %s\n", err)
			return
		}
	} else {
		ai, err = peer.AddrInfoFromP2pAddr(parsed)
		if err != nil {
			fmt.Printf("could not parse peer: %s\n", err)
			return
		}
	}
	fmt.Printf("connecting to %s...\n", ai)

	if err := h.Connect(context.Background(), *ai); err != nil {
		fmt.Printf("could not connect: %s\n", err)
		return
	}
	fmt.Printf("connected...\n")

	protos, err := h.Peerstore().GetProtocols(ai.ID)
	if err != nil {
		fmt.Printf("couldn't get protos: %s\n", err)
		return
	}
	fmt.Printf("protocols: %s\n", protos)
}

const (
	lowWater    = 1200
	highWater   = 1800
	gracePeriod = time.Minute
)

func NewHost(priv crypto.PrivKey, listenAddrs []multiaddr.Multiaddr) (host.Host, error) {
	ps, err := pstoremem.NewPeerstore()
	if err != nil {
		return nil, err
	}

	cmgr, _ := connmgr.NewConnManager(lowWater, highWater)

	libp2pOpts := []libp2p.Option{
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.ConnectionManager(cmgr),
		libp2p.Identity(priv),
		libp2p.EnableNATService(),
		libp2p.Peerstore(ps),
		libp2p.AutoNATServiceRateLimit(0, 3, time.Minute),
		libp2p.DefaultMuxers,
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Security(tls.ID, tls.New),
		libp2p.Security(noise.ID, noise.New),
	}

	node, err := libp2p.New(libp2pOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn libp2p node: %w", err)
	}

	return node, nil
}
