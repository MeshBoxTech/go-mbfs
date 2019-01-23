package helpers

import (
	"context"
	"io"
	"os"

	dag "mbfs/go-mbfs/gx/QmaDBne4KeY3UepeqSVKYpSmQGa3q9zP6x3LfVF2UjF3Hc/go-merkledag"

	ft "mbfs/go-mbfs/gx/QmXLCwhHh7bxRsBnCKNE9BAN87V44aSxXLquZYTtjr6fZ3/go-unixfs"
	pb "mbfs/go-mbfs/gx/QmXLCwhHh7bxRsBnCKNE9BAN87V44aSxXLquZYTtjr6fZ3/go-unixfs/pb"

	chunker "mbfs/go-mbfs/gx/QmR4QQVkBZsZENRjYFVi8dEtPL3daZRNKk24m4r6WKJHNm/go-ipfs-chunker"
	pi "mbfs/go-mbfs/gx/QmR6YMs8EkXQLXNwQKxLnQp2VBZSepoEJ8KCZAyanJHhJu/go-ipfs-posinfo"
	cid "mbfs/go-mbfs/gx/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	files "mbfs/go-mbfs/gx/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"
	ipld "mbfs/go-mbfs/gx/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"

	// added by vingo
	"github.com/ipfs/go-ipfs/core/crypto"
	"crypto/sha256"
)

// DagBuilderHelper wraps together a bunch of objects needed to
// efficiently create unixfs dag trees
type DagBuilderHelper struct {
	dserv      ipld.DAGService
	spl        chunker.Splitter
	recvdErr   error
	rawLeaves  bool
	nextData   []byte // the next item to return.
	maxlinks   int
	cidBuilder cid.Builder

	// Filestore support variables.
	// ----------------------------
	// TODO: Encapsulate in `FilestoreNode` (which is basically what they are).
	//
	// Besides having the path this variable (if set) is used as a flag
	// to indicate that Filestore should be used.
	fullPath string
	stat     os.FileInfo
	// Keeps track of the current file size added to the DAG (used in
	// the balanced builder). It is assumed that the `DagBuilderHelper`
	// is not reused to construct another DAG, but a new one (with a
	// zero `offset`) is created.
	offset uint64

	// added by vingo
	AccessKey []byte
}

// DagBuilderParams wraps configuration options to create a DagBuilderHelper
// from a chunker.Splitter.
type DagBuilderParams struct {
	// Maximum number of links per intermediate node
	Maxlinks int

	// RawLeaves signifies that the importer should use raw ipld nodes as leaves
	// instead of using the unixfs TRaw type
	RawLeaves bool

	// CID Builder to use if set
	CidBuilder cid.Builder

	// DAGService to write blocks to (required)
	Dagserv ipld.DAGService

	// NoCopy signals to the chunker that it should track fileinfo for
	// filestore adds
	NoCopy bool

	// URL if non-empty (and NoCopy is also true) indicates that the
	// file will not be stored in the datastore but instead retrieved
	// from this location via the urlstore.
	URL string

	// added by vingo
	AccessKey []byte
}

// New generates a new DagBuilderHelper from the given params and a given
// chunker.Splitter as data source.
func (dbp *DagBuilderParams) New(spl chunker.Splitter) *DagBuilderHelper {
	db := &DagBuilderHelper{
		dserv:      dbp.Dagserv,
		spl:        spl,
		rawLeaves:  dbp.RawLeaves,
		cidBuilder: dbp.CidBuilder,
		maxlinks:   dbp.Maxlinks,
		// added by vingo
		AccessKey:  dbp.AccessKey,
	}
	if fi, ok := spl.Reader().(files.FileInfo); dbp.NoCopy && ok {
		db.fullPath = fi.AbsPath()
		db.stat = fi.Stat()
	}

	if dbp.URL != "" && dbp.NoCopy {
		db.fullPath = dbp.URL
	}
	return db
}

//////////////////////////// common //////////////////////////////////

// Next returns the next chunk of data to be inserted into the dag
// if it returns nil, that signifies that the stream is at an end, and
// that the current building operation should finish.
// 取下一个数据块用来插入到 dag，如果返回 nil 则说明当前数据已经处理完了
func (db *DagBuilderHelper) Next() ([]byte, error) {
	db.prepareNext() // idempotent
	d := db.nextData
	db.nextData = nil // signal we've consumed it
	if db.recvdErr != nil {
		return nil, db.recvdErr
	}
	return d, nil
}

// prepareNext consumes the next item from the splitter and puts it
// in the nextData field. it is idempotent-- if nextData is full
// it will do nothing.
// 如果db.nextData 有数据，则将 db.nextData 返回，否则从db.spl 读一块数据放入 db.nextData
func (db *DagBuilderHelper) prepareNext() {
	// if we already have data waiting to be consumed, we're ready
	if db.nextData != nil || db.recvdErr != nil {
		return
	}

	// 这里会调用 sizeSplitterv2 的 NextBytes() 方法从 io.Reader 正式读取文件数据，最大读取 spl.size 个字节
	db.nextData, db.recvdErr = db.spl.NextBytes()
	if db.recvdErr == io.EOF {
		db.recvdErr = nil
	}
}

// Done returns whether or not we're done consuming the incoming data.
func (db *DagBuilderHelper) Done() bool {
	// ensure we have an accurate perspective on data
	// as `done` this may be called before `next`.
	db.prepareNext() // idempotent
	if db.recvdErr != nil {
		return false
	}
	return db.nextData == nil
}

// AddUnixfsNode sends a node to the DAGService, and returns it as ipld.Node.
func (db *DagBuilderHelper) AddUnixfsNode(node *UnixfsNode) (ipld.Node, error) {
	dn, err := node.GetDagNode()
	if err != nil {
		return nil, err
	}

	err = db.dserv.Add(context.TODO(), dn)
	if err != nil {
		return nil, err
	}

	return dn, nil
}


// NewUnixfsNode creates a new Unixfs node to represent a file.
func (db *DagBuilderHelper) NewUnixfsNode() *UnixfsNode {
	n := &UnixfsNode{
		node: new(dag.ProtoNode),
		ufmt: ft.NewFSNode(ft.TFile),
	}
	n.SetCidBuilder(db.cidBuilder)
	return n
}


// Maxlinks returns the configured maximum number for links
// for nodes built with this helper.
func (db *DagBuilderHelper) Maxlinks() int {
	return db.maxlinks
}



/////////////////////////////// balance layout ////////////////////////////

// NewLeafDataNode is a variation of `GetNextDataNode` that returns
// an `ipld.Node` instead. It builds the `node` with the data obtained
// from the Splitter and returns it with the `dataSize` (that will be
// used to keep track of the DAG file size). The size of the data is
// computed here because after that it will be hidden by `NewLeafNode`
// inside a generic `ipld.Node` representation.
//
func (db *DagBuilderHelper) NewLeafDataNode() (node ipld.Node, dataSize uint64, err error) {
	// 如果db.nextData 有数据，则将 db.nextData 返回，并重置db.nextData=nil，
	// 否则先从db.spl 读一块数据再做以上处理
	fileData, err := db.Next()
	if err != nil {
		return nil, 0, err
	}

	// added by vingo 对数据块进行加密
	if len(db.AccessKey) > 0 && db.AccessKey != nil{
		key := sha256.Sum256(db.AccessKey)
		fileData = crypto.AesEncrypt(fileData, key[:])
	}
	////////

	dataSize = uint64(len(fileData))

	// Create a new leaf node containing the file chunk data.
	node, err = db.NewLeafNode(fileData)
	if err != nil {
		return nil, 0, err
	}

	// Convert this leaf to a `FilestoreNode` if needed.
	node = db.ProcessFileStore(node, dataSize)

	return node, dataSize, nil
}

// ProcessFileStore generates, if Filestore is being used, the
// `FilestoreNode` representation of the `ipld.Node` that
// contains the file data. If Filestore is not being used just
// return the same node to continue with its addition to the DAG.
//
// The `db.offset` is updated at this point (instead of when
// `NewLeafDataNode` is called, both work in tandem but the
// offset is more related to this function).
func (db *DagBuilderHelper) ProcessFileStore(node ipld.Node, dataSize uint64) ipld.Node {
	// Check if Filestore is being used.
	if db.fullPath != "" {
		// Check if the node is actually a raw node (needed for
		// Filestore support).
		if _, ok := node.(*dag.RawNode); ok {
			fn := &pi.FilestoreNode{
				Node: node,
				PosInfo: &pi.PosInfo{
					Offset:   db.offset,
					FullPath: db.fullPath,
					Stat:     db.stat,
				},
			}

			// Update `offset` with the size of the data generated by `db.Next`.
			db.offset += dataSize

			return fn
		}
	}

	// Filestore is not used, return the same `node` argument.
	return node
}

// NewLeafNode is a variation from `NewLeaf` (see its description) that
// returns an `ipld.Node` instead.
// NewLeafNode是‘newLeaf`(请参阅它的描述)的变体，它返回一个’ipld.Node‘。
func (db *DagBuilderHelper) NewLeafNode(data []byte) (ipld.Node, error) {
	if len(data) > BlockSizeLimit {
		return nil, ErrSizeLimitExceeded
	}

	if db.rawLeaves {
		// Encapsulate the data in a raw node.
		if db.cidBuilder == nil {
			return dag.NewRawNode(data), nil
		}
		rawnode, err := dag.NewRawNodeWPrefix(data, db.cidBuilder)
		if err != nil {
			return nil, err
		}
		return rawnode, nil
	}

	// Encapsulate the data in UnixFS node (instead of a raw node).
	fsNodeOverDag := db.NewFSNodeOverDag(ft.TFile)
	fsNodeOverDag.SetFileData(data)
	// 在 Commit 里面就会将 fsNodeOverDag.file.format 对应的 Data数据结构
	// 通过 Marshal() 进行 protobuf 格式的编码了，这个 Data 数据结构中包含了前面读到的数据 data
	node, err := fsNodeOverDag.Commit()
	if err != nil {
		return nil, err
	}
	// TODO: Encapsulate this sequence of calls into a function that
	// just returns the final `ipld.Node` avoiding going through
	// `FSNodeOverDag`.
	// TODO: Using `TFile` for backwards-compatibility, a bug in the
	// balanced builder was causing the leaf nodes to be generated
	// with this type instead of `TRaw`, the one that should be used
	// (like the trickle builder does).
	// (See https://github.com/ipfs/go-ipfs/pull/5120.)

	return node, nil
}


// Add inserts the given node in the DAGService.
func (db *DagBuilderHelper) Add(node ipld.Node) error {
	return db.dserv.Add(context.TODO(), node)
}


// GetCidBuilder returns the internal `cid.CidBuilder` set in the builder.
func (db *DagBuilderHelper) GetCidBuilder() cid.Builder {
	return db.cidBuilder
}

// NewFSNodeOverDag creates a new `dag.ProtoNode` and `ft.FSNode`
// decoupled from one onther (and will continue in that way until
// `Commit` is called), with `fsNodeType` specifying the type of
// the UnixFS layer node (either `File` or `Raw`).
func (db *DagBuilderHelper) NewFSNodeOverDag(fsNodeType pb.Data_DataType) *FSNodeOverDag {
	node := new(FSNodeOverDag)
	node.dag = new(dag.ProtoNode)
	node.dag.SetCidBuilder(db.GetCidBuilder())

	node.file = ft.NewFSNode(fsNodeType)

	return node
}


// added by vingo  对整个文件数据先进行加密再送入后续流程
// 读取所有数据并对数据加密后放入EncryptedData 成员变量中
//func (db *DagBuilderHelper) EncryptData() bool {
//
//	if db.AccessKey == ""{
//		return false
//	}
//
//	var buf []byte
//	var err error
//
//	for {
//		buf, err = db.spl.NextBytes()
//		if len(buf) > 0 {
//			db.encryptedData = append(db.encryptedData, buf...)
//		}
//
//		if err == io.EOF {
//			db.recvdErr = nil
//			break
//		}
//	}
//
//	//fmt.Println(reflect.TypeOf(db.encryptedData))
//
//	db.encryptedData = crypto.AesEncrypt(db.encryptedData, db.AccessKey)
//	db.low = 0
//	db.max = uint32(len(db.encryptedData))
//
//	return db.encryptedData != nil && db.max > 0
//}
//
//func (db *DagBuilderHelper) prepareNext() {
//	if db.nextData != nil || db.recvdErr != nil {
//		return
//	}
//
//	if db.AccessKey == ""{
//		// 这里会调用 sizeSplitterv2 的 NextBytes() 方法从 io.Reader 正式读取文件数据，最大读取 spl.size 个字节
//		db.nextData, db.recvdErr = db.spl.NextBytes()
//		if db.recvdErr == io.EOF {
//			db.recvdErr = nil
//		}
//	} else {
//
//		if db.high == db.max {
//			db.nextData = nil
//			return
//		}
//
//		var size uint32
//		size = db.spl.GetChunkerSize()
//
//		if db.max-db.high < size {
//			db.high = db.max
//		} else {
//			db.high += size
//		}
//
//		db.nextData = db.encryptedData[db.low:db.high:db.max]
//		if db.high == db.max {
//			db.recvdErr = nil
//			db.low = db.max
//		} else {
//			db.low += size
//		}
//	}
//}
/////////////


// FSNodeOverDag encapsulates an `unixfs.FSNode` that will be stored in a
// `dag.ProtoNode`. Instead of just having a single `ipld.Node` that
// would need to be constantly (un)packed to access and modify its
// internal `FSNode` in the process of creating a UnixFS DAG, this
// structure stores an `FSNode` cache to manipulate it (add child nodes)
// directly , and only when the node has reached its final (immutable) state
// (signaled by calling `Commit()`) is it committed to a single (indivisible)
// `ipld.Node`.
//
// It is used mainly for internal (non-leaf) nodes, and for some
// representations of data leaf nodes (that don't use raw nodes or
// Filestore).
//
// It aims to replace the `UnixfsNode` structure which encapsulated too
// many possible node state combinations.
//
// TODO: Revisit the name.
type FSNodeOverDag struct {
	dag  *dag.ProtoNode
	file *ft.FSNode
}

// AddChild adds a `child` `ipld.Node` to both node layers. The
// `dag.ProtoNode` creates a link to the child node while the
// `ft.FSNode` stores its file size (that is, not the size of the
// node but the size of the file data that it is storing at the
// UnixFS layer). The child is also stored in the `DAGService`.
func (n *FSNodeOverDag) AddChild(child ipld.Node, fileSize uint64, db *DagBuilderHelper) error {
	err := n.dag.AddNodeLink("", child)
	if err != nil {
		return err
	}

	n.file.AddBlockSize(fileSize)

	return db.Add(child)
}

// Commit unifies (resolves) the cache nodes into a single `ipld.Node`
// that represents them: the `ft.FSNode` is encoded inside the
// `dag.ProtoNode`.
//
// TODO: Evaluate making it read-only after committing.
func (n *FSNodeOverDag) Commit() (ipld.Node, error) {
	fileData, err := n.file.GetBytes()
	if err != nil {
		return nil, err
	}
	n.dag.SetData(fileData)

	return n.dag, nil
}

// NumChildren returns the number of children of the `ft.FSNode`.
func (n *FSNodeOverDag) NumChildren() int {
	return n.file.NumChildren()
}

// FileSize returns the `Filesize` attribute from the underlying
// representation of the `ft.FSNode`.
func (n *FSNodeOverDag) FileSize() uint64 {
	return n.file.FileSize()
}

// SetFileData stores the `fileData` in the `ft.FSNode`. It
// should be used only when `FSNodeOverDag` represents a leaf
// node (internal nodes don't carry data, just file sizes).
func (n *FSNodeOverDag) SetFileData(fileData []byte) {
	n.file.SetData(fileData)
}


///////////////////////////  trickledag layout ///////////////////////

// NewLeaf creates a leaf node filled with data.  If rawLeaves is
// defined than a raw leaf will be returned.  Otherwise, if data is
// nil the type field will be TRaw (for backwards compatibility), if
// data is defined (but possibly empty) the type field will be TRaw.
func (db *DagBuilderHelper) NewLeaf(data []byte) (*UnixfsNode, error) {
	if len(data) > BlockSizeLimit {
		return nil, ErrSizeLimitExceeded
	}

	if db.rawLeaves {
		if db.cidBuilder == nil {
			return &UnixfsNode{
				rawnode: dag.NewRawNode(data),
				raw:     true,
			}, nil
		}
		rawnode, err := dag.NewRawNodeWPrefix(data, db.cidBuilder)
		if err != nil {
			return nil, err
		}
		return &UnixfsNode{
			rawnode: rawnode,
			raw:     true,
		}, nil
	}

	if data == nil {
		return db.NewUnixfsNode(), nil
	}

	blk := db.newUnixfsBlock()
	blk.SetData(data)
	return blk, nil
}

// FillNodeLayer will add datanodes as children to the give node until
// at most db.indirSize nodes are added.
func (db *DagBuilderHelper) FillNodeLayer(node *UnixfsNode) error {

	// while we have room AND we're not done
	for node.NumChildren() < db.maxlinks && !db.Done() {
		child, err := db.GetNextDataNode()
		if err != nil {
			return err
		}

		if err := node.AddChild(child, db); err != nil {
			return err
		}
	}

	return nil
}

// GetNextDataNode builds a UnixFsNode with the data obtained from the
// Splitter, given the constraints (BlockSizeLimit, RawLeaves) specified
// when creating the DagBuilderHelper.
func (db *DagBuilderHelper) GetNextDataNode() (*UnixfsNode, error) {
	data, err := db.Next()
	if err != nil {
		return nil, err
	}

	if data == nil { // we're done!
		return nil, nil
	}

	return db.NewLeaf(data)
}



// newUnixfsBlock creates a new Unixfs node to represent a raw data block
func (db *DagBuilderHelper) newUnixfsBlock() *UnixfsNode {
	n := &UnixfsNode{
		node: new(dag.ProtoNode),
		ufmt: ft.NewFSNode(ft.TRaw),
	}
	n.SetCidBuilder(db.cidBuilder)
	return n
}


// GetDagServ returns the dagservice object this Helper is using
func (db *DagBuilderHelper) GetDagServ() ipld.DAGService {
	return db.dserv
}
