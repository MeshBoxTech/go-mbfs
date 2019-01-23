package ping_test

import (
	"context"
	"testing"
	"time"

	pstore "mbfs/go-mbfs/gx/QmUymf8fJtideyv3z727BcZUifGBjMZMpCJqu3Gxk5aRUk/go-libp2p-peerstore"
	bhost "mbfs/go-mbfs/gx/QmXnpYYg2onGLXVxM4Q5PEFcx29k8zeJQkPeLAk9h9naxg/go-libp2p/p2p/host/basic"
	ping "mbfs/go-mbfs/gx/QmXnpYYg2onGLXVxM4Q5PEFcx29k8zeJQkPeLAk9h9naxg/go-libp2p/p2p/protocol/ping"
	swarmt "mbfs/go-mbfs/gx/QmcYC4ayKi7bq8xecEZxHVEuTL6HREZWTTErrSRd1S3Spz/go-libp2p-swarm/testing"
	peer "mbfs/go-mbfs/gx/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
)

func TestPing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h1 := bhost.New(swarmt.GenSwarm(t, ctx))
	h2 := bhost.New(swarmt.GenSwarm(t, ctx))

	err := h1.Connect(ctx, pstore.PeerInfo{
		ID:    h2.ID(),
		Addrs: h2.Addrs(),
	})

	if err != nil {
		t.Fatal(err)
	}

	ps1 := ping.NewPingService(h1)
	ps2 := ping.NewPingService(h2)

	testPing(t, ps1, h2.ID())
	testPing(t, ps2, h1.ID())
}

func testPing(t *testing.T, ps *ping.PingService, p peer.ID) {
	pctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ts, err := ps.Ping(pctx, p)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		select {
		case took := <-ts:
			t.Log("ping took: ", took)
		case <-time.After(time.Second * 4):
			t.Fatal("failed to receive ping")
		}
	}

}
