package routinghelpers

import (
	"context"
	"testing"

	routing "mbfs/go-mbfs/gx/QmYyg3UnyiQubxjs4uhKixPxR7eeKrhJ5Vyz6Et4Tet18B/go-libp2p-routing"
	peert "mbfs/go-mbfs/gx/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer/test"
)

func TestGetPublicKey(t *testing.T) {
	d := Parallel{
		Routers: []routing.IpfsRouting{
			Parallel{
				Routers: []routing.IpfsRouting{
					&Compose{
						ValueStore: &LimitedValueStore{
							ValueStore: new(dummyValueStore),
							Namespaces: []string{"other"},
						},
					},
				},
			},
			Tiered{
				Routers: []routing.IpfsRouting{
					&Compose{
						ValueStore: &LimitedValueStore{
							ValueStore: new(dummyValueStore),
							Namespaces: []string{"pk"},
						},
					},
				},
			},
			&Compose{
				ValueStore: &LimitedValueStore{
					ValueStore: new(dummyValueStore),
					Namespaces: []string{"other", "pk"},
				},
			},
			&struct{ Compose }{Compose{ValueStore: &LimitedValueStore{ValueStore: Null{}}}},
			&struct{ Compose }{},
		},
	}

	pid, _ := peert.RandPeerID()

	ctx := context.Background()
	if _, err := d.GetPublicKey(ctx, pid); err != routing.ErrNotFound {
		t.Fatal(err)
	}
}
