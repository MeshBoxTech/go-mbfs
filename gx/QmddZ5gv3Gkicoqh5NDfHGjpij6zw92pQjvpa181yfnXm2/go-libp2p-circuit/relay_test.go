package relay_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	. "mbfs/go-mbfs/gx/QmddZ5gv3Gkicoqh5NDfHGjpij6zw92pQjvpa181yfnXm2/go-libp2p-circuit"
	pb "mbfs/go-mbfs/gx/QmddZ5gv3Gkicoqh5NDfHGjpij6zw92pQjvpa181yfnXm2/go-libp2p-circuit/pb"

	ma "mbfs/go-mbfs/gx/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	host "mbfs/go-mbfs/gx/QmVrjR2KMe57y4YyfHdYa3yKD278gN8W7CTiqSuYmxjA7F/go-libp2p-host"
	bhost "mbfs/go-mbfs/gx/QmVzYgadebqVb6iCVvjGfgZ7HB9zoCnDgBjMqFLD5aMrFX/go-libp2p-blankhost"
	swarm "mbfs/go-mbfs/gx/QmcYC4ayKi7bq8xecEZxHVEuTL6HREZWTTErrSRd1S3Spz/go-libp2p-swarm"
	swarmt "mbfs/go-mbfs/gx/QmcYC4ayKi7bq8xecEZxHVEuTL6HREZWTTErrSRd1S3Spz/go-libp2p-swarm/testing"
)

/* TODO: add tests
- simple A -[R]-> B
- A tries to relay through R, R doesnt support relay
- A tries to relay through R to B, B doesnt support relay
- A sends too long multiaddr
- R drops stream mid-message
- A relays through R, R has no connection to B
*/

func getNetHosts(t *testing.T, ctx context.Context, n int) []host.Host {
	var out []host.Host

	for i := 0; i < n; i++ {
		netw := swarmt.GenSwarm(t, ctx)
		h := bhost.NewBlankHost(netw)
		out = append(out, h)
	}

	return out
}

func newTestRelay(t *testing.T, ctx context.Context, host host.Host, opts ...RelayOpt) *Relay {
	r, err := NewRelay(ctx, host, swarmt.GenUpgrader(host.Network().(*swarm.Swarm)), opts...)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func connect(t *testing.T, a, b host.Host) {
	pinfo := a.Peerstore().PeerInfo(a.ID())
	err := b.Connect(context.Background(), pinfo)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBasicRelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 3)

	connect(t, hosts[0], hosts[1])
	connect(t, hosts[1], hosts[2])

	time.Sleep(10 * time.Millisecond)

	r1 := newTestRelay(t, ctx, hosts[0], OptDiscovery)

	newTestRelay(t, ctx, hosts[1], OptHop)

	r3 := newTestRelay(t, ctx, hosts[2])

	msg := []byte("relay works!")
	go func() {
		list := r3.Listener()

		con, err := list.Accept()
		if err != nil {
			t.Error(err)
			return
		}

		_, err = con.Write(msg)
		if err != nil {
			t.Error(err)
			return
		}
		con.Close()
	}()

	rinfo := hosts[1].Peerstore().PeerInfo(hosts[1].ID())
	dinfo := hosts[2].Peerstore().PeerInfo(hosts[2].ID())

	rctx, rcancel := context.WithTimeout(ctx, time.Second)
	defer rcancel()

	con, err := r1.DialPeer(rctx, rinfo, dinfo)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(con)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, msg) {
		t.Fatal("message was incorrect:", string(data))
	}
}

func TestRelayReset(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 3)

	connect(t, hosts[0], hosts[1])
	connect(t, hosts[1], hosts[2])

	time.Sleep(10 * time.Millisecond)

	r1 := newTestRelay(t, ctx, hosts[0], OptDiscovery)

	newTestRelay(t, ctx, hosts[1], OptHop)

	r3 := newTestRelay(t, ctx, hosts[2])

	ready := make(chan struct{})

	msg := []byte("relay works!")
	go func() {
		list := r3.Listener()

		con, err := list.Accept()
		if err != nil {
			t.Error(err)
			return
		}

		<-ready

		_, err = con.Write(msg)
		if err != nil {
			t.Error(err)
			return
		}

		hosts[2].Network().ClosePeer(hosts[1].ID())
	}()

	rinfo := hosts[1].Peerstore().PeerInfo(hosts[1].ID())
	dinfo := hosts[2].Peerstore().PeerInfo(hosts[2].ID())

	rctx, rcancel := context.WithTimeout(ctx, time.Second)
	defer rcancel()

	con, err := r1.DialPeer(rctx, rinfo, dinfo)
	if err != nil {
		t.Fatal(err)
	}

	close(ready)

	_, err = ioutil.ReadAll(con)
	if err == nil {
		t.Fatal("expected error for reset relayed connection")
	}
}

func TestBasicRelayDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 3)

	connect(t, hosts[0], hosts[1])
	connect(t, hosts[1], hosts[2])

	time.Sleep(10 * time.Millisecond)

	r1 := newTestRelay(t, ctx, hosts[0], OptDiscovery)

	_ = newTestRelay(t, ctx, hosts[1], OptHop)
	r3 := newTestRelay(t, ctx, hosts[2])

	msg := []byte("relay works!")
	go func() {
		list := r3.Listener()

		con, err := list.Accept()
		if err != nil {
			t.Error(err)
			return
		}

		_, err = con.Write(msg)
		if err != nil {
			t.Error(err)
			return
		}
		con.Close()
	}()

	addr := ma.StringCast(fmt.Sprintf("/ipfs/%s/p2p-circuit", hosts[1].ID().Pretty()))

	rctx, rcancel := context.WithTimeout(ctx, time.Second)
	defer rcancel()

	con, err := r1.Dial(rctx, addr, hosts[2].ID())
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(con)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, msg) {
		t.Fatal("message was incorrect:", string(data))
	}
}

func TestUnspecificRelayDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 3)

	r1 := newTestRelay(t, ctx, hosts[0], OptDiscovery)

	newTestRelay(t, ctx, hosts[1], OptHop)

	r3 := newTestRelay(t, ctx, hosts[2])

	connect(t, hosts[0], hosts[1])
	connect(t, hosts[1], hosts[2])

	time.Sleep(100 * time.Millisecond)

	msg := []byte("relay works!")
	go func() {
		list := r3.Listener()

		con, err := list.Accept()
		if err != nil {
			t.Error(err)
			return
		}

		_, err = con.Write(msg)
		if err != nil {
			t.Error(err)
			return
		}
		con.Close()
	}()

	addr := ma.StringCast(fmt.Sprintf("/p2p-circuit"))

	rctx, rcancel := context.WithTimeout(ctx, time.Second)
	defer rcancel()

	con, err := r1.Dial(rctx, addr, hosts[2].ID())
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(con)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, msg) {
		t.Fatal("message was incorrect:", string(data))
	}
}

func TestRelayThroughNonHop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 3)

	connect(t, hosts[0], hosts[1])
	connect(t, hosts[1], hosts[2])

	time.Sleep(10 * time.Millisecond)

	r1 := newTestRelay(t, ctx, hosts[0], OptDiscovery)

	newTestRelay(t, ctx, hosts[1])

	newTestRelay(t, ctx, hosts[2])

	rinfo := hosts[1].Peerstore().PeerInfo(hosts[1].ID())
	dinfo := hosts[2].Peerstore().PeerInfo(hosts[2].ID())

	rctx, rcancel := context.WithTimeout(ctx, time.Second)
	defer rcancel()

	_, err := r1.DialPeer(rctx, rinfo, dinfo)
	if err == nil {
		t.Fatal("expected error")
	}

	rerr, ok := err.(RelayError)
	if !ok {
		t.Fatalf("expected RelayError: %#v", err)
	}

	if rerr.Code != pb.CircuitRelay_HOP_CANT_SPEAK_RELAY {
		t.Fatal("expected 'HOP_CANT_SPEAK_RELAY' error")
	}
}

func TestRelayNoDestConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 3)

	connect(t, hosts[0], hosts[1])

	time.Sleep(10 * time.Millisecond)

	r1 := newTestRelay(t, ctx, hosts[0], OptDiscovery)

	newTestRelay(t, ctx, hosts[1], OptHop)

	rinfo := hosts[1].Peerstore().PeerInfo(hosts[1].ID())
	dinfo := hosts[2].Peerstore().PeerInfo(hosts[2].ID())

	rctx, rcancel := context.WithTimeout(ctx, time.Second)
	defer rcancel()

	_, err := r1.DialPeer(rctx, rinfo, dinfo)
	if err == nil {
		t.Fatal("expected error")
	}

	rerr, ok := err.(RelayError)
	if !ok {
		t.Fatalf("expected RelayError: %#v", err)
	}

	if rerr.Code != pb.CircuitRelay_HOP_NO_CONN_TO_DST {
		t.Fatal("expected 'HOP_NO_CONN_TO_DST' error")
	}
}

func TestActiveRelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 3)

	connect(t, hosts[0], hosts[1])

	time.Sleep(10 * time.Millisecond)

	r1 := newTestRelay(t, ctx, hosts[0], OptDiscovery)

	newTestRelay(t, ctx, hosts[1], OptHop, OptActive)

	r3 := newTestRelay(t, ctx, hosts[2])

	msg := []byte("relay works!")
	go func() {
		list := r3.Listener()

		con, err := list.Accept()
		if err != nil {
			t.Error(err)
			return
		}

		_, err = con.Write(msg)
		if err != nil {
			t.Error(err)
			return
		}
		con.Close()
	}()

	rinfo := hosts[1].Peerstore().PeerInfo(hosts[1].ID())
	dinfo := hosts[2].Peerstore().PeerInfo(hosts[2].ID())

	rctx, rcancel := context.WithTimeout(ctx, time.Second)
	defer rcancel()

	con, err := r1.DialPeer(rctx, rinfo, dinfo)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(con)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, msg) {
		t.Fatal("message was incorrect:", string(data))
	}
}

func TestRelayCanHop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hosts := getNetHosts(t, ctx, 2)

	connect(t, hosts[0], hosts[1])

	time.Sleep(10 * time.Millisecond)

	r1 := newTestRelay(t, ctx, hosts[0])

	newTestRelay(t, ctx, hosts[1], OptHop)

	canhop, err := r1.CanHop(ctx, hosts[1].ID())
	if err != nil {
		t.Fatal(err)
	}

	if !canhop {
		t.Fatal("Relay can't hop")
	}
}
