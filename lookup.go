package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	ds "github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

func findPeer(h host.Host, pid peer.ID) (*peer.AddrInfo, error) {
	bp := dht.DefaultBootstrapPeers

	if len(bp) > 0 {
		fmt.Printf("bootstrapping...\n")
		var wg sync.WaitGroup
		wg.Add(len(bp))
		for _, addr := range bp {
			go func(addr multiaddr.Multiaddr) {
				defer wg.Done()
				ai, err := peer.AddrInfoFromP2pAddr(addr)
				if err != nil {
					return
				}
				if err := h.Connect(context.Background(), *ai); err != nil {
					return
				}
				h.ConnManager().Protect(ai.ID, "bootstrap-peer")
			}(addr)
		}
		wg.Wait()
	}
	fmt.Printf("starting DHT...\n")

	tmpDS := dssync.MutexWrap(ds.NewMapDatastore())
	dhtOpts := []dht.Option{
		dht.Mode(dht.ModeClient),
		dht.ProtocolPrefix("/ipfs"),
		dht.BucketSize(20),
		dht.Datastore(tmpDS),
		dht.QueryFilter(dht.PublicQueryFilter),
		dht.RoutingTableFilter(dht.PublicRoutingTableFilter),
	}
	dhtNode, err := dht.New(context.Background(), h, dhtOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate DHT: %w", err)
	}
	dhtNode.Bootstrap(context.Background())

	ai, err := dhtNode.FindPeer(context.Background(), pid)
	if err != nil {
		return nil, err
	}

	return &ai, nil
}
