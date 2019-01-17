package coreapi

import (
	"context"
	"fmt"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/filestore"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"
	"github.com/ipfs/go-ipfs/core/coreunix"

	"gx/ipfs/QmPpnbwgAuvhUkA9jGooR88ZwZtTUHXXvoQNKdjZC6nYku/go-ipfs-exchange-offline"
	bstore "gx/ipfs/QmSNLNnL3kq3A1NGdQA9AtgxM9CWKiiSEup3W435jCkRQS/go-ipfs-blockstore"
	"gx/ipfs/QmVPeMNK9DfGLXDZzs2W4RoFWC9Zq1EnLGmLXtYtWrNdcW/go-blockservice"
	ft "gx/ipfs/QmXLCwhHh7bxRsBnCKNE9BAN87V44aSxXLquZYTtjr6fZ3/go-unixfs"
	uio "gx/ipfs/QmXLCwhHh7bxRsBnCKNE9BAN87V44aSxXLquZYTtjr6fZ3/go-unixfs/io"
	"gx/ipfs/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"
	dag "gx/ipfs/QmaDBne4KeY3UepeqSVKYpSmQGa3q9zP6x3LfVF2UjF3Hc/go-merkledag"
	dagtest "gx/ipfs/QmaDBne4KeY3UepeqSVKYpSmQGa3q9zP6x3LfVF2UjF3Hc/go-merkledag/test"
	"gx/ipfs/QmbfKu17LbMWyGUxHEUns9Wf5Dkm8PT6be4uPhTkk4YvaV/go-cidutil"
	ipld "gx/ipfs/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
	"gx/ipfs/QmcUXFi2Fp7oguoFT81f2poJpnb44dFkZanQhDBHMoYyG9/go-mfs"

	// added by vingo
	"github.com/ipfs/go-ipfs/core/crypto"
	mdag "gx/ipfs/QmaDBne4KeY3UepeqSVKYpSmQGa3q9zP6x3LfVF2UjF3Hc/go-merkledag"
	"gx/ipfs/QmdxUuburamoF6zF9qjeQC4WYcWGbWuRmdLacMEsW8ioD8/gogo-protobuf/proto"
	pb "gx/ipfs/QmXLCwhHh7bxRsBnCKNE9BAN87V44aSxXLquZYTtjr6fZ3/go-unixfs/pb"
	"crypto/sha256"
)

type UnixfsAPI CoreAPI

// Add builds a merkledag node from a reader, adds it to the blockstore,
// and returns the key representing that node.
func (api *UnixfsAPI) Add(ctx context.Context, files files.File, opts ...options.UnixfsAddOption) (coreiface.ResolvedPath, error) {
	// 根据 API 接收到的Option（参数）给setting 赋值，并初始化文件编码的前缀相关信息
	settings, prefix, err := options.UnixfsAddOptions(opts...)
	if err != nil {
		return nil, err
	}

	n := api.node

	cfg, err := n.Repo.Config()
	if err != nil {
		return nil, err
	}

	// check if repo will exceed storage limit if added
	// TODO: this doesn't handle the case if the hashed file is already in blocks (deduplicated)
	// TODO: conditional GC is disabled due to it is somehow not possible to pass the size to the daemon
	//if err := corerepo.ConditionalGC(req.Context(), n, uint64(size)); err != nil {
	//	res.SetError(err, cmdkit.ErrNormal)
	//	return
	//}

	if settings.NoCopy && !cfg.Experimental.FilestoreEnabled {
		return nil, filestore.ErrFilestoreNotEnabled
	}

	if settings.OnlyHash {
		nilnode, err := core.NewNode(ctx, &core.BuildCfg{
			//TODO: need this to be true or all files
			// hashed will be stored in memory!
			NilRepo: true,
		})
		if err != nil {
			return nil, err
		}
		n = nilnode
	}

	addblockstore := n.Blockstore
	if !(settings.FsCache || settings.NoCopy) {
		addblockstore = bstore.NewGCBlockstore(n.BaseBlocks, n.GCLocker)
	}

	exch := n.Exchange
	if settings.Local {
		exch = offline.Exchange(addblockstore)
	}

	bserv := blockservice.New(addblockstore, exch) // hash security 001
	dserv := dag.NewDAGService(bserv)

	fileAdder, err := coreunix.NewAdder(ctx, n.Pinning, n.Blockstore, dserv)
	if err != nil {
		return nil, err
	}

	fileAdder.Chunker = settings.Chunker
	if settings.Events != nil {
		fileAdder.Out = settings.Events
		fileAdder.Progress = settings.Progress
	}
	fileAdder.Hidden = settings.Hidden
	fileAdder.Wrap = settings.Wrap
	fileAdder.Pin = settings.Pin && !settings.OnlyHash
	fileAdder.Silent = settings.Silent
	fileAdder.RawLeaves = settings.RawLeaves
	fileAdder.NoCopy = settings.NoCopy
	fileAdder.Name = settings.StdinName
	fileAdder.CidBuilder = prefix

	// added by vingo
	fileAdder.AccessKey = settings.AccessKey
	////////////////

	switch settings.Layout {
		case options.BalancedLayout:
			// Default
		case options.TrickleLayout:
			fileAdder.Trickle = true
		default:
			return nil, fmt.Errorf("unknown layout: %d", settings.Layout)
	}

	if settings.Inline {
		fileAdder.CidBuilder = cidutil.InlineBuilder{
			Builder: fileAdder.CidBuilder,
			Limit:   settings.InlineLimit,
		}
	}

	if settings.OnlyHash {
		md := dagtest.Mock()
		emptyDirNode := ft.EmptyDirNode()
		// Use the same prefix for the "empty" MFS root as for the file adder.
		emptyDirNode.SetCidBuilder(fileAdder.CidBuilder)
		mr, err := mfs.NewRoot(ctx, md, emptyDirNode, nil)
		if err != nil {
			return nil, err
		}

		fileAdder.SetMfsRoot(mr)
	}

	nd, err := fileAdder.AddAllAndPin(files)
	if err != nil {
		return nil, err
	}

	return coreiface.IpfsPath(nd.Cid()), nil
}

//func (api *UnixfsAPI) Get(ctx context.Context, p coreiface.Path) (coreiface.UnixfsFile, error) {
// added by vingo
func (api *UnixfsAPI) Get(ctx context.Context, p coreiface.Path, accKey []byte) (coreiface.UnixfsFile, error) {
	ses := api.core().getSession(ctx)
	nd, err := ses.ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	// added by vingo
	var pbd pb.Data
	var rawData []byte
	n := nd.(*mdag.ProtoNode)
	rawData = n.Data()

	// 如果添加文件时设置了密码才进行解密处理
	if len(n.AccessKey) > 0 {
		// 密码不正确，返回错误
		key := sha256.Sum256(accKey)
		if !crypto.IsKeyEqual(key[:], n.AccessKey) {
			return nil,	fmt.Errorf("Incorrect access key：%s", string(accKey))   // TODO: 看看怎么把错误信息返回到 Client 端
		}

		// 小文件直接在这里解密
		if len(nd.Links()) <= 0 {
			// 先对 data 解码
			proto.Unmarshal(rawData, &pbd)
			// 再对 data 解密
			origData := crypto.AesDecrypt(pbd.Data, n.AccessKey)
			// 解密失败，可能是数据已经被破坏
			if origData == nil {
				return nil,	fmt.Errorf("Decrypt data error：%s", p)
			}

			pbd.Data = origData
			pbd.Filesize = proto.Uint64(uint64(len(origData)))

			// 再将解密后的 data 编码回去
			decoded, err := proto.Marshal(&pbd)
			if err != nil {
				return nil, err
			}
			// 将重新编码后的解密数据放回 ProtoNode 的 data 字段
			n.SetData(decoded)

		} else {
			// 设置解密大文件 Chunker 节点的密码，用于在 pbdagreader 中的 loadBufNode() 方法中读取 Chunker 节点数据时进行解密
			for _, lnk := range nd.Links() {
				if crypto.DecryptKey == nil {
					crypto.Init()
				}
				//fmt.Println(lnk.Cid.String(), "====Set link's acckey====", string(accKey))
				crypto.DecryptKey[lnk.Cid.String()] = n.AccessKey
			}
		}
	}
	////////////////

	return newUnixfsFile(ctx, ses.dag, nd, "", nil)
}

// Ls returns the contents of an IPFS or IPNS object(s) at path p, with the format:
// `<link base58 hash> <link size in bytes> <link name>`
func (api *UnixfsAPI) Ls(ctx context.Context, p coreiface.Path) ([]*ipld.Link, error) {
	dagnode, err := api.core().ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	var ndlinks []*ipld.Link
	dir, err := uio.NewDirectoryFromNode(api.dag, dagnode)
	switch err {
	case nil:
		l, err := dir.Links(ctx)
		if err != nil {
			return nil, err
		}
		ndlinks = l
	case uio.ErrNotADir:
		ndlinks = dagnode.Links()
	default:
		return nil, err
	}

	links := make([]*ipld.Link, len(ndlinks))
	for i, l := range ndlinks {
		links[i] = &ipld.Link{Name: l.Name, Size: l.Size, Cid: l.Cid}
	}
	return links, nil
}

func (api *UnixfsAPI) core() *CoreAPI {
	return (*CoreAPI)(api)
}
