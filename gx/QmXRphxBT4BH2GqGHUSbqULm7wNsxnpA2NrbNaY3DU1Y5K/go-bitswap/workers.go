package bitswap

import (
	"context"
	"math/rand"
	"sync"
	"time"

	bsmsg "mbfs/go-mbfs/gx/QmXRphxBT4BH2GqGHUSbqULm7wNsxnpA2NrbNaY3DU1Y5K/go-bitswap/message"

	"mbfs/go-mbfs/gx/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	process "mbfs/go-mbfs/gx/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess"
	procctx "mbfs/go-mbfs/gx/QmSF8fPo3jgVBAy8fpdjjYqgG87dkJgUprRBHRd2tmfgpP/goprocess/context"
	"mbfs/go-mbfs/gx/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	logging "mbfs/go-mbfs/gx/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
)

var TaskWorkerCount = 8

func (bs *Bitswap) startWorkers(px process.Process, ctx context.Context) {
	// Start up a worker to handle block requests this node is making
	px.Go(func(px process.Process) {
		bs.providerQueryManager(ctx)
	})

	// Start up workers to handle requests from other nodes for the data on this node
	for i := 0; i < TaskWorkerCount; i++ {
		i := i
		px.Go(func(px process.Process) {
			bs.taskWorker(ctx, i)
		})
	}

	// Start up a worker to manage periodically resending our wantlist out to peers
	px.Go(func(px process.Process) {
		bs.rebroadcastWorker(ctx)
	})

	// Start up a worker to manage sending out provides messages
	px.Go(func(px process.Process) {
		bs.provideCollector(ctx)
	})

	// Spawn up multiple workers to handle incoming blocks
	// consider increasing number if providing blocks bottlenecks
	// file transfers
	px.Go(bs.provideWorker)

	// added by vingo
	if bs.addprovs != nil {
		px.Go(func(px process.Process) {
			bs.getBlocksOfProviders(ctx)
		})
	}
	//////////////////

}

// 通过 engine 的 taskWorker 协程循环去 peerRequestQueue 队列取出需要发送的 block
// 将 block 封装成 envelope 对象后发送给需要的节点
func (bs *Bitswap) taskWorker(ctx context.Context, id int) {
	idmap := logging.LoggableMap{"ID": id}
	defer log.Debug("bitswap task worker shutting down...")
	for {
		log.Event(ctx, "Bitswap.TaskWorker.Loop", idmap)
		select {
		case nextEnvelope := <-bs.engine.Outbox():
			select {
				case envelope, ok := <-nextEnvelope:
					if !ok {
						continue
					}
					// update the BS ledger to reflect sent message
					// TODO: Should only track *useful* messages in ledger
					outgoing := bsmsg.New(false)
					for _, block := range envelope.Message.Blocks() {
						log.Event(ctx, "Bitswap.TaskWorker.Work", logging.LoggableF(func() map[string]interface{} {
							return logging.LoggableMap{
								"ID":     id,
								"Target": envelope.Peer.Pretty(),
								"Block":  block.Cid().String(),
							}
						}))
						outgoing.AddBlock(block)
					}
					bs.engine.MessageSent(envelope.Peer, outgoing)

					bs.wm.SendBlocks(ctx, envelope)
					bs.counterLk.Lock()
					for _, block := range envelope.Message.Blocks() {
						bs.counters.blocksSent++
						bs.counters.dataSent += uint64(len(block.RawData()))
					}
					bs.counterLk.Unlock()
				case <-ctx.Done():
					return
			}
		case <-ctx.Done():
			return
		}
	}
}

// 监听  provideKeys 管道收到的 cid ，然后通过 routing 层将其广播到网络
func (bs *Bitswap) provideWorker(px process.Process) {

	limit := make(chan struct{}, provideWorkerMax)

	limitedGoProvide := func(k cid.Cid, wid int) {
		defer func() {
			// replace token when done
			<-limit
		}()
		ev := logging.LoggableMap{"ID": wid}

		ctx := procctx.OnClosingContext(px) // derive ctx from px
		defer log.EventBegin(ctx, "Bitswap.ProvideWorker.Work", ev, k).Done()

		ctx, cancel := context.WithTimeout(ctx, provideTimeout) // timeout ctx
		defer cancel()

		// 将刚收到的 k(block 的 cid) 通过 bitSwap 的 routing 广播到网络
		if err := bs.network.Provide(ctx, k); err != nil {
			log.Warning(err)
		}
	}

	// worker spawner, reads from bs.provideKeys until it closes, spawning a
	// _ratelimited_ number of workers to handle each key.
	for wid := 2; ; wid++ {
		ev := logging.LoggableMap{"ID": 1}
		log.Event(procctx.OnClosingContext(px), "Bitswap.ProvideWorker.Loop", ev)

		select {
		case <-px.Closing():
			return
		case k, ok := <-bs.provideKeys:
			if !ok {
				log.Debug("provideKeys channel closed")
				return
			}
			select {
			case <-px.Closing():
				return
			case limit <- struct{}{}:
				go limitedGoProvide(k, wid)		// 启用协程处理从 provideKeys 管道传入的 cid
			}
		}
	}
}

// 监听  newBlocks 管道传入的新收到的 cid ，然后将其通过 provideKeys 管道传给 provideWorker
// provideWorker会将其通过 routing 层广播到网络
func (bs *Bitswap) provideCollector(ctx context.Context) {
	defer close(bs.provideKeys)
	var toProvide []cid.Cid
	var nextKey cid.Cid
	var keysOut chan cid.Cid

	for {
		select {
			case blkey, ok := <-bs.newBlocks:	// receiveBlockFrom收到新的 block 后会通过 NewBlocks 信道传过来
				if !ok {
					log.Debug("newBlocks channel closed")
					return
				}

				if keysOut == nil {
					nextKey = blkey
					keysOut = bs.provideKeys
				} else {
					toProvide = append(toProvide, blkey)
				}
			case keysOut <- nextKey:			// 在这里实际上已经将 newBlocks 管道传入的 cid 传入了 provideKeys 管道
				if len(toProvide) > 0 {
					nextKey = toProvide[0]
					toProvide = toProvide[1:]
				} else {
					keysOut = nil
				}
			case <-ctx.Done():
				return
		}
	}
}

// 用计时器控制周期性地从WantManager的指定 peer 的队列中随机选中一个 cid 放入 findKeys 管道，
// 然后由 providerQueryManager 协程去查找其对应的 provider peer
func (bs *Bitswap) rebroadcastWorker(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	broadcastSignal := time.NewTicker(rebroadcastDelay.Get())
	defer broadcastSignal.Stop()

	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()

	for {
		log.Event(ctx, "Bitswap.Rebroadcast.idle")
		select {
		case <-tick.C:
			n := bs.wm.wl.Len()
			if n > 0 {
				log.Debug(n, " keys in bitswap wantlist")
			}
		case <-broadcastSignal.C: // resend unfulfilled wantlist keys
			log.Event(ctx, "Bitswap.Rebroadcast.active")
			entries := bs.wm.wl.Entries()
			if len(entries) == 0 {
				continue
			}

			// TODO: come up with a better strategy for determining when to search
			// for new providers for blocks.
			// 从WantManager的指定 peer 的队列中随机选中一个 cid 放入 findKeys 管道去查找其对应的 provider peer
			i := rand.Intn(len(entries))
			bs.findKeys <- &blockRequest{
				Cid: entries[i].Cid,
				Ctx: ctx,
			}
		case <-parent.Done():
			return
		}
	}
}

// 监听 findKeys 管道传入的查询cid 的需求，然后通过 routing 层进行查询
func (bs *Bitswap) providerQueryManager(ctx context.Context) {
	var activeLk sync.Mutex
	kset := cid.NewSet()

	for {
		select {
			case e := <-bs.findKeys:
				select { // make sure its not already cancelled
					case <-e.Ctx.Done():
						continue
					default:
				}

				activeLk.Lock()
				if kset.Has(e.Cid) {
					activeLk.Unlock()
					continue
				}

				kset.Add(e.Cid)
				activeLk.Unlock()

				go func(e *blockRequest) {
					child, cancel := context.WithTimeout(e.Ctx, providerRequestTimeout)
					defer cancel()
					providers := bs.network.FindProvidersAsync(child, e.Cid, maxProvidersPerRequest)
					wg := &sync.WaitGroup{}
					for p := range providers {
						wg.Add(1)
						go func(p peer.ID) {
							defer wg.Done()
							err := bs.network.ConnectTo(child, p)
							if err != nil {
								log.Debug("failed to connect to provider %s: %s", p, err)
							}
						}(p)
					}
					wg.Wait()
					activeLk.Lock()
					kset.Remove(e.Cid)
					activeLk.Unlock()
				}(e)

			case <-ctx.Done():
				return
		}
	}
}


// added by vingo
func (bs *Bitswap) getBlocksOfProviders(ctx context.Context)  {

	for{
		select {
			case key := <- bs.addprovs:
				go bs.GetBlock(ctx, key)
			case <-ctx.Done():
				return
		}
	}
}