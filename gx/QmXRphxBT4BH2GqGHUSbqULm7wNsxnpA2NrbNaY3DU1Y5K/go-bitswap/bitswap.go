// package bitswap implements the IPFS exchange interface with the BitSwap
// bilateral exchange protocol.
package bitswap

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"mbfs/go-mbfs/gx/QmXRphxBT4BH2GqGHUSbqULm7wNsxnpA2NrbNaY3DU1Y5K/go-bitswap/decision"
	bsmsg "mbfs/go-mbfs/gx/QmXRphxBT4BH2GqGHUSbqULm7wNsxnpA2NrbNaY3DU1Y5K/go-bitswap/message"
	bsnet "mbfs/go-mbfs/gx/QmXRphxBT4BH2GqGHUSbqULm7wNsxnpA2NrbNaY3DU1Y5K/go-bitswap/network"
	"mbfs/go-mbfs/gx/QmXRphxBT4BH2GqGHUSbqULm7wNsxnpA2NrbNaY3DU1Y5K/go-bitswap/notifications"

	"mbfs/go-mbfs/gx/QmP2g3VxmC7g7fyRJDj1VJ72KHZbJ9UW24YjSWEj1XTb4H/go-ipfs-exchange-interface"
	"mbfs/go-mbfs/gx/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"mbfs/go-mbfs/gx/QmRJVNatYJwTAHgdSM1Xef9QVQ1Ch3XHdmcrykjP5Y4soL/go-ipfs-delay"
	"mbfs/go-mbfs/gx/QmRMGdC6HKdLsPDABL9aXPDidrpmEHzJqFWSvshkbn9Hj8/go-ipfs-flags"
	process "mbfs/go-mbfs/gx/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	procctx "mbfs/go-mbfs/gx/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess/context"
	"mbfs/go-mbfs/gx/QmSNLNnL3kq3A1NGdQA9AtgxM9CWKiiSEup3W435jCkRQS/go-ipfs-blockstore"
	"mbfs/go-mbfs/gx/QmWoXtvgC8inqFkAATB7cp2Dax7XBi9VDvSg9RCCZufmRk/go-block-format"
	"mbfs/go-mbfs/gx/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	logging "mbfs/go-mbfs/gx/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
	"mbfs/go-mbfs/gx/QmekzFM3hPZjTjUFGTABdQkEnQ3PTiMstY198PwSFr5w1Q/go-metrics-interface"
)

var log = logging.Logger("bitswap")

var _ exchange.SessionExchange = (*Bitswap)(nil)

const (
	// maxProvidersPerRequest specifies the maximum number of providers desired
	// from the network. This value is specified because the network streams
	// results.
	// TODO: if a 'non-nice' strategy is implemented, consider increasing this value
	maxProvidersPerRequest = 3
	findProviderDelay      = 1 * time.Second
	providerRequestTimeout = time.Second * 10
	provideTimeout         = time.Second * 15
	sizeBatchRequestChan   = 32
	// kMaxPriority is the max priority as defined by the bitswap protocol
	kMaxPriority = math.MaxInt32
)

var (
	HasBlockBufferSize    = 256
	provideKeysBufferSize = 2048
	provideWorkerMax      = 512

	// the 1<<18+15 is to observe old file chunks that are 1<<18 + 14 in size
	metricsBuckets = []float64{1 << 6, 1 << 10, 1 << 14, 1 << 18, 1<<18 + 15, 1 << 22}
)

func init() {
	if flags.LowMemMode {
		HasBlockBufferSize = 64
		provideKeysBufferSize = 512
		provideWorkerMax = 16
	}
}

var rebroadcastDelay = delay.Fixed(time.Minute)

// New initializes a BitSwap instance that communicates over the provided
// BitSwapNetwork. This function registers the returned instance as the network
// delegate.
// Runs until context is cancelled.
func New(parent context.Context, net bsnet.BitSwapNetwork, bstore blockstore.Blockstore) exchange.Interface {

	// important to use provided parent context (since it may include important
	// loggable data). It's probably not a good idea to allow bitswap to be
	// coupled to the concerns of the ipfs daemon in this way.
	//
	// FIXME(btc) Now that bitswap manages itself using a process, it probably
	// shouldn't accept a context anymore. Clients should probably use Close()
	// exclusively. We should probably find another way to share logging data
	ctx, cancelFunc := context.WithCancel(parent)
	ctx = metrics.CtxSubScope(ctx, "bitswap")
	dupHist := metrics.NewCtx(ctx, "recv_dup_blocks_bytes", "Summary of duplicate"+" data blocks recived").Histogram(metricsBuckets)
	allHist := metrics.NewCtx(ctx, "recv_all_blocks_bytes", "Summary of all"+" data blocks recived").Histogram(metricsBuckets)

	notif := notifications.New()
	px := process.WithTeardown(func() error {
		notif.Shutdown()
		return nil
	})

	bs := &Bitswap{
		blockstore:    bstore,
		notifications: notif,
		engine:        decision.NewEngine(ctx, bstore), // TODO close the engine with Close() method
		network:       net,
		findKeys:      make(chan *blockRequest, sizeBatchRequestChan),
		process:       px,
		newBlocks:     make(chan cid.Cid, HasBlockBufferSize),
		provideKeys:   make(chan cid.Cid, provideKeysBufferSize),
		wm:            NewWantManager(ctx, net),
		counters:      new(counters),

		dupMetric: dupHist,
		allMetric: allHist,

		// added by vingo
		addprovs:      make(chan cid.Cid, 10),
		/////////////////
	}
	go bs.wm.Run()
	net.SetDelegate(bs)

	// added by vingo
	bs.addprovs = net.SubCopyProvs()
	//if n, ok := net.(network)
	//if router, ok := bsnet.routing.(*dht.IpfsDHT); ok {
	//	prov := router.GetProviders()
	//	if bs, y := r.(*bitswap.Bitswap); y{
	//		bs.SetProvChan(prov.SubCopyProvs())
	//	}
	//}
	/////////////////

	// Start up bitswaps async worker routines
	bs.startWorkers(px, ctx)

	// bind the context and process.
	// do it over here to avoid closing before all setup is done.
	go func() {
		<-px.Closing() // process closes first
		cancelFunc()
	}()
	procctx.CloseAfterContext(px, ctx) // parent cancelled first

	return bs
}

// Bitswap instances implement the bitswap protocol.
type Bitswap struct {
	// the peermanager manages sending messages to peers in a way that
	// wont block bitswap operation
	wm *WantManager

	// the engine is the bit of logic that decides who to send which blocks to
	engine *decision.Engine

	// network delivers messages on behalf of the session
	network bsnet.BitSwapNetwork

	// blockstore is the local database
	// NB: ensure threadsafety
	blockstore blockstore.Blockstore

	// notifications engine for receiving new blocks and routing them to the
	// appropriate user requests
	notifications notifications.PubSub

	// findKeys sends keys to a worker to find and connect to providers for them
	findKeys chan *blockRequest
	// newBlocks is a channel for newly added blocks to be provided to the
	// network.  blocks pushed down this channel get buffered and fed to the
	// provideKeys channel later on to avoid too much network activity
	newBlocks chan cid.Cid
	// provideKeys directly feeds provide workers
	provideKeys chan cid.Cid

	process process.Process

	// Counters for various statistics
	counterLk sync.Mutex
	counters  *counters

	// Metrics interface metrics
	dupMetric metrics.Histogram
	allMetric metrics.Histogram

	// Sessions
	sessions []*Session
	sessLk   sync.Mutex

	sessID   uint64
	sessIDLk sync.Mutex
	
	// added by vingo
	addprovs chan cid.Cid
}
// added by vingo
type addProv struct {
	k   cid.Cid
	val peer.ID
}
//////////////

type counters struct {
	blocksRecvd    uint64
	dupBlocksRecvd uint64
	dupDataRecvd   uint64
	blocksSent     uint64
	dataSent       uint64
	dataRecvd      uint64
	messagesRecvd  uint64
}

type blockRequest struct {
	Cid cid.Cid
	Ctx context.Context
}

// GetBlock attempts to retrieve a particular block from peers within the
// deadline enforced by the context.
func (bs *Bitswap) GetBlock(parent context.Context, k cid.Cid) (blocks.Block, error) {
	return getBlock(parent, k, bs.GetBlocks)
}

func (bs *Bitswap) WantlistForPeer(p peer.ID) []cid.Cid {
	var out []cid.Cid
	for _, e := range bs.engine.WantlistForPeer(p) {
		out = append(out, e.Cid)
	}
	return out
}

func (bs *Bitswap) LedgerForPeer(p peer.ID) *decision.Receipt {
	return bs.engine.LedgerForPeer(p)
}

// GetBlocks returns a channel where the caller may receive blocks that
// correspond to the provided |keys|. Returns an error if BitSwap is unable to
// begin this request within the deadline enforced by the context.
//
// NB: Your request remains open until the context expires. To conserve
// resources, provide a context with a reasonably short deadline (ie. not one
// that lasts throughout the lifetime of the server)
func (bs *Bitswap) GetBlocks(ctx context.Context, keys []cid.Cid) (<-chan blocks.Block, error) {
	if len(keys) == 0 {
		out := make(chan blocks.Block)
		close(out)
		return out, nil
	}

	select {
	case <-bs.process.Closing():
		return nil, errors.New("bitswap is closed")
	default:
	}
	// 订阅从 notifications 异步返回的管道消息，以便 block 从远端 peer 返回时能及时处理
	promise := bs.notifications.Subscribe(ctx, keys...)

	for _, k := range keys {
		log.Event(ctx, "Bitswap.GetBlockRequest.Start", k)
	}

	mses := bs.getNextSessionID()

	// 将需要 get 的 block 的 cid 加入wantlist，交由 wantmanager 去广播给网络
	bs.wm.WantBlocks(ctx, keys, nil, mses)

	remaining := cid.NewSet()
	for _, k := range keys {
		remaining.Add(k)
	}

	out := make(chan blocks.Block)
	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer close(out)
		defer func() {
			// can't just defer this call on its own, arguments are resolved *when* the defer is created
			bs.CancelWants(remaining.Keys(), mses)
		}()
		findProvsDelay := time.NewTimer(findProviderDelay)
		defer findProvsDelay.Stop()

		findProvsDelayCh := findProvsDelay.C
		req := &blockRequest{
			Cid: keys[0],
			Ctx: ctx,
		}

		var findProvsReqCh chan<- *blockRequest

		for {
			select {
				case <-findProvsDelayCh:		// 超时处理
					// NB: Optimization. Assumes that providers of key[0] are likely to
					// be able to provide for all keys. This currently holds true in most
					// every situation. Later, this assumption may not hold as true.
					findProvsReqCh = bs.findKeys
					findProvsDelayCh = nil
				case findProvsReqCh <- req:		// 将keys[0]间接推入 bs.findKeys 管道，交由 worker.providerQueryManager 协程去处理
					findProvsReqCh = nil
				case blk, ok := <-promise:		// 收到了异步返回的 block
					if !ok {
						return
					}

					// No need to find providers now.
					findProvsDelay.Stop()
					findProvsDelayCh = nil
					findProvsReqCh = nil

					bs.CancelWants([]cid.Cid{blk.Cid()}, mses)
					remaining.Remove(blk.Cid())
					select {
						case out <- blk:		// 将收到的 block 推入 out 管道，以便该方法的调用方从 out 管道取出 block 进行处理
						case <-ctx.Done():
							return
					}
				case <-ctx.Done():
					return
			}
		}
	}()

	return out, nil
}

func (bs *Bitswap) getNextSessionID() uint64 {
	bs.sessIDLk.Lock()
	defer bs.sessIDLk.Unlock()
	bs.sessID++
	return bs.sessID
}

// CancelWant removes a given key from the wantlist
func (bs *Bitswap) CancelWants(cids []cid.Cid, ses uint64) {
	if len(cids) == 0 {
		return
	}
	bs.wm.CancelWants(context.Background(), cids, nil, ses)
}

// HasBlock announces the existence of a block to this bitswap service. The
// service will potentially notify its peers.
// 向交换服务宣布是否存在数据块。服务可能会通知其它节点。
func (bs *Bitswap) HasBlock(blk blocks.Block) error {
	return bs.receiveBlockFrom(blk, "")
}

// TODO: Some of this stuff really only needs to be done when adding a block
// from the user, not when receiving it from the network.
// In case you run `git blame` on this comment, I'll save you some time: ask
// @whyrusleeping, I don't know the answers you seek.
// 其中一些工作实际上只需要在添加来自用户的数据块时完成，而不是在从网络接收数据块时执行。
// 处理本地节点添加的 Block （在AddBlock里面会有调用）
// 或者是从其他节点发过来的 Block（在ReceiveMessage里面会有调用）.
func (bs *Bitswap) receiveBlockFrom(blk blocks.Block, from peer.ID) error {
	select {
	case <-bs.process.Closing():
		return errors.New("bitswap is closed")
	default:
	}

	// 将收到的 block 存入本地数据库（会先检查库中是否已经存在）
	err := bs.blockstore.Put(blk)
	if err != nil {
		log.Errorf("Error writing block to datastore: %s", err)
		return err
	}

	// NOTE: There exists the possiblity for a race condition here.  If a user
	// creates a node, then adds it to the dagservice while another goroutine
	// is waiting on a GetBlock for that object, they will receive a reference
	// to the same node. We should address this soon, but i'm not going to do
	// it now as it requires more thought and isnt causing immediate problems.
	// 这里存在竞赛条件的可能性。如果用户创建了一个 node ，然后将其添加到 dagservice 中，
	// 而另一个goroutine正在为该对象等待GetBlock，则他们将收到对同一 node 的引用。
	// 我们应该尽快解决这一问题，但现在还不会这么做，因为这需要更多的思考，而且这个问题现在还不会有什么影响。

	// 通过将 blk 推入 PubSub 的 cmdChan 通道，
	// 将收到的 block publish 给所有订阅了该 block 的节点
	bs.notifications.Publish(blk)

	k := blk.Cid()
	ks := []cid.Cid{k}
	for _, s := range bs.SessionsForBlock(k) {
		s.receiveBlockFrom(from, blk)
		bs.CancelWants(ks, s.id)
	}

	// 查找一下Engine中缓存的WantList信息，如果还有其它节点也在请求本节点刚刚接收到的数据，
	// 则将该数据放入peerRequestQueue中去
	bs.engine.AddBlock(blk)

	// 将刚收到的 Block 的 cid  通过 newBlocks 管道发送到  worker 的 provideCollector 协程，
	// 随后由 provideCollector 协程通过 providerKeys 管道传到 worker 的 provideWorker 协程，
	// provideWorker 协程会负责将 cid 广播到网络
	select {
		case bs.newBlocks <- blk.Cid():
		case <-bs.process.Closing():
			return bs.process.Close()
	}
	return nil
}

// SessionsForBlock returns a slice of all sessions that may be interested in the given cid
func (bs *Bitswap) SessionsForBlock(c cid.Cid) []*Session {
	bs.sessLk.Lock()
	defer bs.sessLk.Unlock()

	var out []*Session
	for _, s := range bs.sessions {
		if s.interestedIn(c) {
			out = append(out, s)
		}
	}
	return out
}

func (bs *Bitswap) ReceiveMessage(ctx context.Context, p peer.ID, incoming bsmsg.BitSwapMessage) {
	atomic.AddUint64(&bs.counters.messagesRecvd, 1)

	// This call records changes to wantlists, blocks received,
	// and number of bytes transfered.
	// 通过Engine的一个账单系统，统计一下本节点与发送数据节点之间的数据交互统计，
	// 然后再查找一下Engine中缓存的WantList信息，如果还有其它节点也在请求本节点刚刚接收到的数据，
	// 则将该数据放入peerRequestQueue中去
	bs.engine.MessageReceived(p, incoming)

	// TODO: this is bad, and could be easily abused.(这样写确实该鄙视,优化一下，从上面的MessageReceived方法调用里直接返回 iblocks 就好了)
	// Should only track *useful* messages in ledger
	iblocks := incoming.Blocks()
	if len(iblocks) == 0 {
		return
	}

	wg := sync.WaitGroup{}
	for _, block := range iblocks {
		wg.Add(1)
		go func(b blocks.Block) { // TODO: this probably doesnt need to be a goroutine...
			defer wg.Done()

			bs.updateReceiveCounters(b)

			log.Debugf("got block %s from %s", b, p)

			if err := bs.receiveBlockFrom(b, p); err != nil {
				log.Warningf("ReceiveMessage recvBlockFrom error: %s", err)
			}
			log.Event(ctx, "Bitswap.GetBlockRequest.End", b.Cid())
		}(block)
	}
	wg.Wait()
}

var ErrAlreadyHaveBlock = errors.New("already have block")

func (bs *Bitswap) updateReceiveCounters(b blocks.Block) {
	blkLen := len(b.RawData())
	has, err := bs.blockstore.Has(b.Cid())
	if err != nil {
		log.Infof("blockstore.Has error: %s", err)
		return
	}

	bs.allMetric.Observe(float64(blkLen))
	if has {
		bs.dupMetric.Observe(float64(blkLen))
	}

	bs.counterLk.Lock()
	defer bs.counterLk.Unlock()
	c := bs.counters

	c.blocksRecvd++
	c.dataRecvd += uint64(len(b.RawData()))
	if has {
		c.dupBlocksRecvd++
		c.dupDataRecvd += uint64(blkLen)
	}
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerConnected(p peer.ID) {
	bs.wm.Connected(p)
	bs.engine.PeerConnected(p)
}

// Connected/Disconnected warns bitswap about peer connections
func (bs *Bitswap) PeerDisconnected(p peer.ID) {
	bs.wm.Disconnected(p)
	bs.engine.PeerDisconnected(p)
}

func (bs *Bitswap) ReceiveError(err error) {
	log.Infof("Bitswap ReceiveError: %s", err)
	// TODO log the network error
	// TODO bubble the network error up to the parent context/error logger
}

func (bs *Bitswap) Close() error {
	return bs.process.Close()
}

func (bs *Bitswap) GetWantlist() []cid.Cid {
	entries := bs.wm.wl.Entries()
	out := make([]cid.Cid, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Cid)
	}
	return out
}

func (bs *Bitswap) IsOnline() bool {
	return true
}
