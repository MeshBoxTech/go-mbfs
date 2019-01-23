package pstoremem

import (
	"testing"

	pstore "mbfs/go-mbfs/gx/QmUymf8fJtideyv3z727BcZUifGBjMZMpCJqu3Gxk5aRUk/go-libp2p-peerstore"
	pt "mbfs/go-mbfs/gx/QmUymf8fJtideyv3z727BcZUifGBjMZMpCJqu3Gxk5aRUk/go-libp2p-peerstore/test"
)

func TestInMemoryPeerstore(t *testing.T) {
	pt.TestPeerstore(t, func() (pstore.Peerstore, func()) {
		return NewPeerstore(), nil
	})
}

func TestInMemoryAddrBook(t *testing.T) {
	pt.TestAddrBook(t, func() (pstore.AddrBook, func()) {
		return NewAddrBook(), nil
	})
}

func TestInMemoryKeyBook(t *testing.T) {
	pt.TestKeyBook(t, func() (pstore.KeyBook, func()) {
		return NewKeyBook(), nil
	})
}

func BenchmarkInMemoryPeerstore(b *testing.B) {
	pt.BenchmarkPeerstore(b, func() (pstore.Peerstore, func()) {
		return NewPeerstore(), nil
	}, "InMem")
}
