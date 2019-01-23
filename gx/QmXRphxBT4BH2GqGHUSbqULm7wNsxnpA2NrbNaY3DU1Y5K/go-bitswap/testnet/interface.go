package bitswap

import (
	bsnet "mbfs/go-mbfs/gx/QmXRphxBT4BH2GqGHUSbqULm7wNsxnpA2NrbNaY3DU1Y5K/go-bitswap/network"
	"mbfs/go-mbfs/gx/QmZXjR5X1p4KrQ967cTsy4MymMzUM8mZECF3PV8UcN4o3g/go-testutil"
	peer "mbfs/go-mbfs/gx/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
)

type Network interface {
	Adapter(testutil.Identity) bsnet.BitSwapNetwork

	HasPeer(peer.ID) bool
}
