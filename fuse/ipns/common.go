package ipns

import (
	"context"

	"mbfs/go-mbfs/core"
	nsys "mbfs/go-mbfs/namesys"
	ci "mbfs/go-mbfs/gx/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	path "mbfs/go-mbfs/gx/QmRG3XuGwT7GYuAqgWDJBKTzdaHMwAnc1x7J2KHEXNHxzG/go-path"
	ft "mbfs/go-mbfs/gx/QmXLCwhHh7bxRsBnCKNE9BAN87V44aSxXLquZYTtjr6fZ3/go-unixfs"
)

// InitializeKeyspace sets the ipns record for the given key to
// point to an empty directory.
func InitializeKeyspace(n *core.IpfsNode, key ci.PrivKey) error {
	ctx, cancel := context.WithCancel(n.Context())
	defer cancel()

	emptyDir := ft.EmptyDirNode()

	err := n.Pinning.Pin(ctx, emptyDir, false)
	if err != nil {
		return err
	}

	err = n.Pinning.Flush()
	if err != nil {
		return err
	}

	pub := nsys.NewIpnsPublisher(n.Routing, n.Repo.Datastore())

	return pub.Publish(ctx, key, path.FromCid(emptyDir.Cid()))
}
