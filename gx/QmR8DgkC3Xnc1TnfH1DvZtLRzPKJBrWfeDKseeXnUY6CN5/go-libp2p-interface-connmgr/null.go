package ifconnmgr

import (
	"context"

	ma "mbfs/go-mbfs/gx/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	inet "mbfs/go-mbfs/gx/QmRKbEchaYADxSCyyjhDh4cTrUby8ftXUb8MRLBTHQYupw/go-libp2p-net"
	peer "mbfs/go-mbfs/gx/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
)

type NullConnMgr struct{}

func (_ NullConnMgr) TagPeer(peer.ID, string, int)  {}
func (_ NullConnMgr) UntagPeer(peer.ID, string)     {}
func (_ NullConnMgr) GetTagInfo(peer.ID) *TagInfo   { return &TagInfo{} }
func (_ NullConnMgr) TrimOpenConns(context.Context) {}
func (_ NullConnMgr) Notifee() inet.Notifiee        { return &cmNotifee{} }

type cmNotifee struct{}

func (nn *cmNotifee) Connected(n inet.Network, c inet.Conn)         {}
func (nn *cmNotifee) Disconnected(n inet.Network, c inet.Conn)      {}
func (nn *cmNotifee) Listen(n inet.Network, addr ma.Multiaddr)      {}
func (nn *cmNotifee) ListenClose(n inet.Network, addr ma.Multiaddr) {}
func (nn *cmNotifee) OpenedStream(inet.Network, inet.Stream)        {}
func (nn *cmNotifee) ClosedStream(inet.Network, inet.Stream)        {}
