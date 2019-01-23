// Package nilrouting implements a routing client that does nothing.
package nilrouting

import (
	"context"
	"errors"

	cid "mbfs/go-mbfs/gx/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	record "mbfs/go-mbfs/gx/QmSoeYGNm8v8jAF49hX7UwHwkXjoeobSrn9sya5NPPsxXP/go-libp2p-record"
	pstore "mbfs/go-mbfs/gx/QmUymf8fJtideyv3z727BcZUifGBjMZMpCJqu3Gxk5aRUk/go-libp2p-peerstore"
	p2phost "mbfs/go-mbfs/gx/QmVrjR2KMe57y4YyfHdYa3yKD278gN8W7CTiqSuYmxjA7F/go-libp2p-host"
	routing "mbfs/go-mbfs/gx/QmYyg3UnyiQubxjs4uhKixPxR7eeKrhJ5Vyz6Et4Tet18B/go-libp2p-routing"
	ropts "mbfs/go-mbfs/gx/QmYyg3UnyiQubxjs4uhKixPxR7eeKrhJ5Vyz6Et4Tet18B/go-libp2p-routing/options"
	ds "mbfs/go-mbfs/gx/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	peer "mbfs/go-mbfs/gx/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
)

type nilclient struct {
}

func (c *nilclient) PutValue(_ context.Context, _ string, _ []byte, _ ...ropts.Option) error {
	return nil
}

func (c *nilclient) GetValue(_ context.Context, _ string, _ ...ropts.Option) ([]byte, error) {
	return nil, errors.New("tried GetValue from nil routing")
}

func (c *nilclient) SearchValue(_ context.Context, _ string, _ ...ropts.Option) (<-chan []byte, error) {
	return nil, errors.New("tried SearchValue from nil routing")
}

func (c *nilclient) FindPeer(_ context.Context, _ peer.ID) (pstore.PeerInfo, error) {
	return pstore.PeerInfo{}, nil
}

func (c *nilclient) FindProvidersAsync(_ context.Context, _ cid.Cid, _ int) <-chan pstore.PeerInfo {
	out := make(chan pstore.PeerInfo)
	defer close(out)
	return out
}

func (c *nilclient) Provide(_ context.Context, _ cid.Cid, _ bool) error {
	return nil
}

func (c *nilclient) Bootstrap(_ context.Context) error {
	return nil
}

// ConstructNilRouting creates an IpfsRouting client which does nothing.
func ConstructNilRouting(_ context.Context, _ p2phost.Host, _ ds.Batching, _ record.Validator) (routing.IpfsRouting, error) {
	return &nilclient{}, nil
}

//  ensure nilclient satisfies interface
var _ routing.IpfsRouting = &nilclient{}
