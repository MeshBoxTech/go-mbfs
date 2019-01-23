package mdutils

import (
	dag "mbfs/go-mbfs/gx/QmaDBne4KeY3UepeqSVKYpSmQGa3q9zP6x3LfVF2UjF3Hc/go-merkledag"

	offline "mbfs/go-mbfs/gx/QmPpnbwgAuvhUkA9jGooR88ZwZtTUHXXvoQNKdjZC6nYku/go-ipfs-exchange-offline"
	blockstore "mbfs/go-mbfs/gx/QmSNLNnL3kq3A1NGdQA9AtgxM9CWKiiSEup3W435jCkRQS/go-ipfs-blockstore"
	bsrv "mbfs/go-mbfs/gx/QmVPeMNK9DfGLXDZzs2W4RoFWC9Zq1EnLGmLXtYtWrNdcW/go-blockservice"
	ds "mbfs/go-mbfs/gx/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	dssync "mbfs/go-mbfs/gx/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/sync"
	ipld "mbfs/go-mbfs/gx/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
)

// Mock returns a new thread-safe, mock DAGService.
func Mock() ipld.DAGService {
	return dag.NewDAGService(Bserv())
}

// Bserv returns a new, thread-safe, mock BlockService.
func Bserv() bsrv.BlockService {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	return bsrv.New(bstore, offline.Exchange(bstore))
}
