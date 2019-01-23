package host

import pstore "mbfs/go-mbfs/gx/QmUymf8fJtideyv3z727BcZUifGBjMZMpCJqu3Gxk5aRUk/go-libp2p-peerstore"

// PeerInfoFromHost returns a PeerInfo struct with the Host's ID and all of its Addrs.
func PeerInfoFromHost(h Host) *pstore.PeerInfo {
	return &pstore.PeerInfo{
		ID:    h.ID(),
		Addrs: h.Addrs(),
	}
}
