package pstoremem

import (
	"sync"

	pstore "mbfs/go-mbfs/gx/QmUymf8fJtideyv3z727BcZUifGBjMZMpCJqu3Gxk5aRUk/go-libp2p-peerstore"
	peer "mbfs/go-mbfs/gx/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
)

type memoryPeerMetadata struct {
	// store other data, like versions
	//ds ds.ThreadSafeDatastore
	ds     map[string]interface{}
	dslock sync.Mutex
}

var _ pstore.PeerMetadata = (*memoryPeerMetadata)(nil)

func NewPeerMetadata() pstore.PeerMetadata {
	return &memoryPeerMetadata{
		ds: make(map[string]interface{}),
	}
}

func (ps *memoryPeerMetadata) Put(p peer.ID, key string, val interface{}) error {
	//dsk := ds.NewKey(string(p) + "/" + key)
	//return ps.ds.Put(dsk, val)
	ps.dslock.Lock()
	defer ps.dslock.Unlock()
	ps.ds[string(p)+"/"+key] = val
	return nil
}

func (ps *memoryPeerMetadata) Get(p peer.ID, key string) (interface{}, error) {
	//dsk := ds.NewKey(string(p) + "/" + key)
	//return ps.ds.Get(dsk)

	ps.dslock.Lock()
	defer ps.dslock.Unlock()
	i, ok := ps.ds[string(p)+"/"+key]
	if !ok {
		return nil, pstore.ErrNotFound
	}
	return i, nil
}
