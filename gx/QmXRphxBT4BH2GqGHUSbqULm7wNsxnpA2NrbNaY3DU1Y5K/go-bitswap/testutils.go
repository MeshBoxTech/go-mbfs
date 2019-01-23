package bitswap

import (
	"context"
	"time"

	tn "mbfs/go-mbfs/gx/QmXRphxBT4BH2GqGHUSbqULm7wNsxnpA2NrbNaY3DU1Y5K/go-bitswap/testnet"

	delay "mbfs/go-mbfs/gx/QmRJVNatYJwTAHgdSM1Xef9QVQ1Ch3XHdmcrykjP5Y4soL/go-ipfs-delay"
	blockstore "mbfs/go-mbfs/gx/QmSNLNnL3kq3A1NGdQA9AtgxM9CWKiiSEup3W435jCkRQS/go-ipfs-blockstore"
	p2ptestutil "mbfs/go-mbfs/gx/QmVPQuFZTw2yVm5Uu1MGofaNBnqaySQddLHKCFjWyoKGhv/go-libp2p-netutil"
	testutil "mbfs/go-mbfs/gx/QmZXjR5X1p4KrQ967cTsy4MymMzUM8mZECF3PV8UcN4o3g/go-testutil"
	ds "mbfs/go-mbfs/gx/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	delayed "mbfs/go-mbfs/gx/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/delayed"
	ds_sync "mbfs/go-mbfs/gx/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/sync"
	peer "mbfs/go-mbfs/gx/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
)

// WARNING: this uses RandTestBogusIdentity DO NOT USE for NON TESTS!
func NewTestSessionGenerator(
	net tn.Network) SessionGenerator {
	ctx, cancel := context.WithCancel(context.Background())
	return SessionGenerator{
		net:    net,
		seq:    0,
		ctx:    ctx, // TODO take ctx as param to Next, Instances
		cancel: cancel,
	}
}

// TODO move this SessionGenerator to the core package and export it as the core generator
type SessionGenerator struct {
	seq    int
	net    tn.Network
	ctx    context.Context
	cancel context.CancelFunc
}

func (g *SessionGenerator) Close() error {
	g.cancel()
	return nil // for Closer interface
}

func (g *SessionGenerator) Next() Instance {
	g.seq++
	p, err := p2ptestutil.RandTestBogusIdentity()
	if err != nil {
		panic("FIXME") // TODO change signature
	}
	return MkSession(g.ctx, g.net, p)
}

func (g *SessionGenerator) Instances(n int) []Instance {
	var instances []Instance
	for j := 0; j < n; j++ {
		inst := g.Next()
		instances = append(instances, inst)
	}
	for i, inst := range instances {
		for j := i + 1; j < len(instances); j++ {
			oinst := instances[j]
			inst.Exchange.network.ConnectTo(context.Background(), oinst.Peer)
		}
	}
	return instances
}

type Instance struct {
	Peer       peer.ID
	Exchange   *Bitswap
	blockstore blockstore.Blockstore

	blockstoreDelay delay.D
}

func (i *Instance) Blockstore() blockstore.Blockstore {
	return i.blockstore
}

func (i *Instance) SetBlockstoreLatency(t time.Duration) time.Duration {
	return i.blockstoreDelay.Set(t)
}

// session creates a test bitswap instance.
//
// NB: It's easy make mistakes by providing the same peer ID to two different
// sessions. To safeguard, use the SessionGenerator to generate sessions. It's
// just a much better idea.
func MkSession(ctx context.Context, net tn.Network, p testutil.Identity) Instance {
	bsdelay := delay.Fixed(0)

	adapter := net.Adapter(p)
	dstore := ds_sync.MutexWrap(delayed.New(ds.NewMapDatastore(), bsdelay))

	bstore, err := blockstore.CachedBlockstore(ctx,
		blockstore.NewBlockstore(ds_sync.MutexWrap(dstore)),
		blockstore.DefaultCacheOpts())
	if err != nil {
		panic(err.Error()) // FIXME perhaps change signature and return error.
	}

	bs := New(ctx, adapter, bstore).(*Bitswap)

	return Instance{
		Peer:            p.ID(),
		Exchange:        bs,
		blockstore:      bstore,
		blockstoreDelay: bsdelay,
	}
}
