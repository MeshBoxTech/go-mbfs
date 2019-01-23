package blankhost

import (
	"context"
	"io"

	ifconnmgr "mbfs/go-mbfs/gx/QmR8DgkC3Xnc1TnfH1DvZtLRzPKJBrWfeDKseeXnUY6CN5/go-libp2p-interface-connmgr"
	ma "mbfs/go-mbfs/gx/QmRKLtwMw131aK7ugC3G7ybpumMz78YrJe5dzneyindvG1/go-multiaddr"
	inet "mbfs/go-mbfs/gx/QmRKbEchaYADxSCyyjhDh4cTrUby8ftXUb8MRLBTHQYupw/go-libp2p-net"
	pstore "mbfs/go-mbfs/gx/QmUymf8fJtideyv3z727BcZUifGBjMZMpCJqu3Gxk5aRUk/go-libp2p-peerstore"
	host "mbfs/go-mbfs/gx/QmVrjR2KMe57y4YyfHdYa3yKD278gN8W7CTiqSuYmxjA7F/go-libp2p-host"
	protocol "mbfs/go-mbfs/gx/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	mstream "mbfs/go-mbfs/gx/QmabLh8TrJ3emfAoQk5AbqbLTbMyj7XqumMFmAFxa9epo8/go-multistream"
	peer "mbfs/go-mbfs/gx/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	logging "mbfs/go-mbfs/gx/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
)

var log = logging.Logger("blankhost")

// BlankHost is the thinnest implementation of the host.Host interface
type BlankHost struct {
	n    inet.Network
	mux  *mstream.MultistreamMuxer
	cmgr ifconnmgr.ConnManager
}

func NewBlankHost(n inet.Network) *BlankHost {
	bh := &BlankHost{
		n:    n,
		cmgr: &ifconnmgr.NullConnMgr{},
		mux:  mstream.NewMultistreamMuxer(),
	}

	n.SetStreamHandler(bh.newStreamHandler)
	return bh
}

var _ host.Host = (*BlankHost)(nil)

func (bh *BlankHost) Addrs() []ma.Multiaddr {
	addrs, err := bh.n.InterfaceListenAddresses()
	if err != nil {
		log.Debug("error retrieving network interface addrs: ", err)
		return nil
	}

	return addrs
}

func (bh *BlankHost) Close() error {
	return bh.n.Close()
}

func (bh *BlankHost) Connect(ctx context.Context, pi pstore.PeerInfo) error {
	// absorb addresses into peerstore
	bh.Peerstore().AddAddrs(pi.ID, pi.Addrs, pstore.TempAddrTTL)

	cs := bh.n.ConnsToPeer(pi.ID)
	if len(cs) > 0 {
		return nil
	}

	_, err := bh.Network().DialPeer(ctx, pi.ID)
	return err
}

func (bh *BlankHost) Peerstore() pstore.Peerstore {
	return bh.n.Peerstore()
}

func (bh *BlankHost) ID() peer.ID {
	return bh.n.LocalPeer()
}

func (bh *BlankHost) NewStream(ctx context.Context, p peer.ID, protos ...protocol.ID) (inet.Stream, error) {
	s, err := bh.n.NewStream(ctx, p)
	if err != nil {
		return nil, err
	}

	var protoStrs []string
	for _, pid := range protos {
		protoStrs = append(protoStrs, string(pid))
	}

	selected, err := mstream.SelectOneOf(protoStrs, s)
	if err != nil {
		s.Close()
		return nil, err
	}

	selpid := protocol.ID(selected)
	s.SetProtocol(selpid)
	bh.Peerstore().AddProtocols(p, selected)

	return s, nil
}

func (bh *BlankHost) RemoveStreamHandler(p protocol.ID) {
	bh.Mux().RemoveHandler(string(p))
}

func (bh *BlankHost) SetStreamHandler(pid protocol.ID, handler inet.StreamHandler) {
	bh.Mux().AddHandler(string(pid), func(p string, rwc io.ReadWriteCloser) error {
		is := rwc.(inet.Stream)
		is.SetProtocol(protocol.ID(p))
		handler(is)
		return nil
	})
}

func (bh *BlankHost) SetStreamHandlerMatch(pid protocol.ID, m func(string) bool, handler inet.StreamHandler) {
	bh.Mux().AddHandlerWithFunc(string(pid), m, func(p string, rwc io.ReadWriteCloser) error {
		is := rwc.(inet.Stream)
		is.SetProtocol(protocol.ID(p))
		handler(is)
		return nil
	})
}

// newStreamHandler is the remote-opened stream handler for inet.Network
func (h *BlankHost) newStreamHandler(s inet.Stream) {

	protoID, handle, err := h.Mux().Negotiate(s)
	if err != nil {
		log.Warning("protocol mux failed: %s", err)
		s.Close()
		return
	}

	s.SetProtocol(protocol.ID(protoID))

	go handle(protoID, s)
}

// TODO: i'm not sure this really needs to be here
func (bh *BlankHost) Mux() *mstream.MultistreamMuxer {
	return bh.mux
}

// TODO: also not sure this fits... Might be better ways around this (leaky abstractions)
func (bh *BlankHost) Network() inet.Network {
	return bh.n
}

func (bh *BlankHost) ConnManager() ifconnmgr.ConnManager {
	return bh.cmgr
}
