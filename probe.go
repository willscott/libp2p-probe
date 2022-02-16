package main

import (
	"context"
	"fmt"
	"os"

	csms "github.com/libp2p/go-conn-security-multistream"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	noise "github.com/libp2p/go-libp2p-noise"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/transport"
	mplex "github.com/libp2p/go-libp2p-mplex"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	swarm "github.com/libp2p/go-libp2p-swarm"
	tls "github.com/libp2p/go-libp2p-tls"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	yamux "github.com/libp2p/go-libp2p-yamux"
	basichost "github.com/libp2p/go-libp2p/p2p/host/basic"
	msmux "github.com/libp2p/go-stream-muxer-multistream"
	"github.com/libp2p/go-tcp-transport"
	ws "github.com/libp2p/go-ws-transport"
	"github.com/multiformats/go-multiaddr"
)

func main() {
	args := os.Args

	if len(args) < 2 {
		fmt.Printf("usage: probe <multiaddr>\n")
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
	parsed, err := multiaddr.NewMultiaddr(ma)
	if err != nil {
		fmt.Printf("could not parse peer: %s\n", err)
		return
	}
	fmt.Printf("connecting to %s...\n", parsed)

	ai, err := peer.AddrInfoFromP2pAddr(parsed)
	if err != nil {
		fmt.Printf("could not parse peer: %s\n", err)
		return
	}

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

func NewHost(priv crypto.PrivKey, listenAddrs []multiaddr.Multiaddr) (host.Host, error) {
	ps, err := pstoremem.NewPeerstore()
	if err != nil {
		return nil, err
	}
	pub := priv.GetPublic()
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}

	if err := ps.AddPrivKey(pid, priv); err != nil {
		return nil, err
	}
	if err := ps.AddPubKey(pid, pub); err != nil {
		return nil, err
	}

	net, err := swarm.NewSwarm(pid, ps)
	if err != nil {
		return nil, err
	}

	secMuxer := new(csms.SSMuxer)
	noiseSec, _ := noise.New(priv)
	secMuxer.AddTransport(noise.ID, noiseSec)
	tlsSec, _ := tls.New(priv)
	secMuxer.AddTransport(tls.ID, tlsSec)

	muxMuxer := msmux.NewBlankTransport()
	muxMuxer.AddTransport("/yamux/1.0.0", yamux.DefaultTransport)
	muxMuxer.AddTransport("/mplex/6.7.0", mplex.DefaultTransport)

	upgrader, err := tptu.New(secMuxer, muxMuxer)
	if err != nil {
		return nil, err
	}

	tcpT, _ := tcp.NewTCPTransport(upgrader, nil)
	for _, t := range []transport.Transport{
		tcpT,
		ws.New(upgrader, nil),
	} {
		if err := net.AddTransport(t); err != nil {
			return nil, err
		}
	}

	if err := net.Listen(listenAddrs...); err != nil {
		return nil, err
	}

	host, err := basichost.NewHost(net, &basichost.HostOpts{})
	if err != nil {
		return nil, err
	}

	host.Start()
	return host, nil
}
