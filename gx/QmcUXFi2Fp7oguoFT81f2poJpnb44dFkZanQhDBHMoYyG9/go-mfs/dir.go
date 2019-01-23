package mfs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	ft "mbfs/go-mbfs/gx/QmXLCwhHh7bxRsBnCKNE9BAN87V44aSxXLquZYTtjr6fZ3/go-unixfs"
	uio "mbfs/go-mbfs/gx/QmXLCwhHh7bxRsBnCKNE9BAN87V44aSxXLquZYTtjr6fZ3/go-unixfs/io"
	dag "mbfs/go-mbfs/gx/QmaDBne4KeY3UepeqSVKYpSmQGa3q9zP6x3LfVF2UjF3Hc/go-merkledag"

	cid "mbfs/go-mbfs/gx/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	ipld "mbfs/go-mbfs/gx/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
)

var ErrNotYetImplemented = errors.New("not yet implemented")
var ErrInvalidChild = errors.New("invalid child node")
var ErrDirExists = errors.New("directory already has entry by that name")

type Directory struct {
	dserv  ipld.DAGService

	parent childCloser

	childDirs map[string]*Directory
	files     map[string]*File

	lock sync.Mutex
	ctx  context.Context

	// UnixFS directory implementation used for creating,
	// reading and editing directories.
	unixfsDir uio.Directory

	modTime time.Time

	name string

	// added by vingo
	AccessKey []byte
}

// NewDirectory constructs a new MFS directory.
//
// You probably don't want to call this directly. Instead, construct a new root
// using NewRoot.
func NewDirectory(ctx context.Context, name string, node ipld.Node, parent childCloser, dserv ipld.DAGService) (*Directory, error) {
	db, err := uio.NewDirectoryFromNode(dserv, node)
	if err != nil {
		return nil, err
	}

	return &Directory{
		dserv:     dserv,
		ctx:       ctx,
		name:      name,
		unixfsDir: db,
		parent:    parent,
		childDirs: make(map[string]*Directory),
		files:     make(map[string]*File),
		modTime:   time.Now(),
	}, nil
}

// GetCidBuilder gets the CID builder of the root node
func (d *Directory) GetCidBuilder() cid.Builder {
	return d.unixfsDir.GetCidBuilder()
}

// SetCidBuilder sets the CID builder
func (d *Directory) SetCidBuilder(b cid.Builder) {
	d.unixfsDir.SetCidBuilder(b)
}

// closeChild updates the child by the given name to the dag node 'nd'
// and changes its own dag node
func (d *Directory) closeChild(name string, nd ipld.Node, sync bool) error {
	mynd, err := d.closeChildUpdate(name, nd, sync)
	if err != nil {
		return err
	}

	if sync {
		return d.parent.closeChild(d.name, mynd, true)
	}
	return nil
}

// closeChildUpdate is the portion of closeChild that needs to be locked around
func (d *Directory) closeChildUpdate(name string, nd ipld.Node, sync bool) (*dag.ProtoNode, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	err := d.updateChild(name, nd)
	if err != nil {
		return nil, err
	}

	if sync {
		return d.flushCurrentNode()
	}
	return nil, nil
}

func (d *Directory) flushCurrentNode() (*dag.ProtoNode, error) {
	nd, err := d.unixfsDir.GetNode()
	if err != nil {
		return nil, err
	}

	err = d.dserv.Add(d.ctx, nd)
	if err != nil {
		return nil, err
	}

	pbnd, ok := nd.(*dag.ProtoNode)
	if !ok {
		return nil, dag.ErrNotProtobuf
	}

	return pbnd.Copy().(*dag.ProtoNode), nil
}
//  将 name 和 nd 更新到 d 的 unixfsDir中，需要对 nd 重新 encode 并计算 cid
func (d *Directory) updateChild(name string, nd ipld.Node) error {
	err := d.AddUnixFSChild(name, nd)
	if err != nil {
		return err
	}

	d.modTime = time.Now()

	return nil
}

func (d *Directory) Type() NodeType {
	return TDir
}

// childNode returns a FSNode under this directory by the given name if it exists.
// it does *not* check the cached dirs and files
func (d *Directory) childNode(name string) (FSNode, error) {
	nd, err := d.childFromDag(name)
	if err != nil {
		return nil, err
	}

	return d.cacheNode(name, nd)
}

// cacheNode caches a node into d.childDirs or d.files and returns the FSNode.
func (d *Directory) cacheNode(name string, nd ipld.Node) (FSNode, error) {
	switch nd := nd.(type) {
	case *dag.ProtoNode:
		fsn, err := ft.FSNodeFromBytes(nd.Data())
		if err != nil {
			return nil, err
		}

		switch fsn.Type() {
		case ft.TDirectory, ft.THAMTShard:
			ndir, err := NewDirectory(d.ctx, name, nd, d, d.dserv)
			if err != nil {
				return nil, err
			}

			d.childDirs[name] = ndir
			return ndir, nil
		case ft.TFile, ft.TRaw, ft.TSymlink:
			nfi, err := NewFile(name, nd, d, d.dserv)
			if err != nil {
				return nil, err
			}
			d.files[name] = nfi
			return nfi, nil
		case ft.TMetadata:
			return nil, ErrNotYetImplemented
		default:
			return nil, ErrInvalidChild
		}
	case *dag.RawNode:
		nfi, err := NewFile(name, nd, d, d.dserv)
		if err != nil {
			return nil, err
		}
		d.files[name] = nfi
		return nfi, nil
	default:
		return nil, fmt.Errorf("unrecognized node type in cache node")
	}
}

// Child returns the child of this directory by the given name
func (d *Directory) Child(name string) (FSNode, error) {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.childUnsync(name)
}

func (d *Directory) Uncache(name string) {
	d.lock.Lock()
	defer d.lock.Unlock()
	delete(d.files, name)
	delete(d.childDirs, name)
}

// childFromDag searches through this directories dag node for a child link
// with the given name
func (d *Directory) childFromDag(name string) (ipld.Node, error) {
	return d.unixfsDir.Find(d.ctx, name)
}

// childUnsync returns the child under this directory by the given name
// without locking, useful for operations which already hold a lock
func (d *Directory) childUnsync(name string) (FSNode, error) {
	// 从 d 这个 Directory 的 childDirs 中找 name 对应的节点，
	cdir, ok := d.childDirs[name]
	if ok {
		return cdir, nil
	}

	// 从 d 这个 Directory 的 files 中找 name 对应的节点，
	cfile, ok := d.files[name]
	if ok {
		return cfile, nil
	}
	// 从 d 这个 Directory 的 unixfsDir.node.links 中找 name 对应的节点，
	return d.childNode(name)
}

type NodeListing struct {
	Name string
	Type int
	Size int64
	Hash string
}

func (d *Directory) ListNames(ctx context.Context) ([]string, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	var out []string
	err := d.unixfsDir.ForEachLink(ctx, func(l *ipld.Link) error {
		out = append(out, l.Name)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (d *Directory) List(ctx context.Context) ([]NodeListing, error) {
	var out []NodeListing
	err := d.ForEachEntry(ctx, func(nl NodeListing) error {
		out = append(out, nl)
		return nil
	})
	return out, err
}

func (d *Directory) ForEachEntry(ctx context.Context, f func(NodeListing) error) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	return d.unixfsDir.ForEachLink(ctx, func(l *ipld.Link) error {
		c, err := d.childUnsync(l.Name)
		if err != nil {
			return err
		}

		nd, err := c.GetNode()
		if err != nil {
			return err
		}

		child := NodeListing{
			Name: l.Name,
			Type: int(c.Type()),
			Hash: nd.Cid().String(),
		}

		if c, ok := c.(*File); ok {
			size, err := c.Size()
			if err != nil {
				return err
			}
			child.Size = size
		}

		return f(child)
	})
}

func (d *Directory) Mkdir(name string) (*Directory, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	fsn, err := d.childUnsync(name)
	if err == nil {
		switch fsn := fsn.(type) {
		case *Directory:
			return fsn, os.ErrExist
		case *File:
			return nil, os.ErrExist
		default:
			return nil, fmt.Errorf("unrecognized type: %#v", fsn)
		}
	}

	ndir := ft.EmptyDirNode()
	ndir.SetCidBuilder(d.GetCidBuilder())
	err = d.dserv.Add(d.ctx, ndir)
	if err != nil {
		return nil, err
	}

	err = d.AddUnixFSChild(name, ndir)
	if err != nil {
		return nil, err
	}

	dirobj, err := NewDirectory(d.ctx, name, ndir, d, d.dserv)
	if err != nil {
		return nil, err
	}

	d.childDirs[name] = dirobj
	return dirobj, nil
}

func (d *Directory) Unlink(name string) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	delete(d.childDirs, name)
	delete(d.files, name)

	return d.unixfsDir.RemoveChild(d.ctx, name)
}

func (d *Directory) Flush() error {
	nd, err := d.GetNode()
	if err != nil {
		return err
	}

	return d.parent.closeChild(d.name, nd, true)
}

// AddChild adds the node 'nd' under this directory giving it the name 'name'
// 将 nd 节点添加到 name 所对应的 Directory 的 unixfsDir.node.link 中
func (d *Directory) AddChild(name string, nd ipld.Node) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	_, err := d.childUnsync(name)
	if err == nil {
		return ErrDirExists
	}

	// 检查缓存和存储中是否已经有 nd，如果还没有则将 nd 加入缓存和存储
	err = d.dserv.Add(d.ctx, nd)
	if err != nil {
		return err
	}

	// 将 nd 节点添加到 name 所对应的 Directory 的 unixfsDir.node.link 中
	err = d.AddUnixFSChild(name, nd)
	if err != nil {
		return err
	}

	d.modTime = time.Now()
	return nil
}

// AddUnixFSChild adds a child to the inner UnixFS directory
// and transitions to a HAMT implementation if needed.
// 将 nd 节点添加到 name 所 d.unixfsDir['name'] 的 成员变量 node 的 link 中
func (d *Directory) AddUnixFSChild(name string, node ipld.Node) error {
	if uio.UseHAMTSharding {
		// If the directory HAMT implementation is being used and this
		// directory is actually a basic implementation switch it to HAMT.
		if basicDir, ok := d.unixfsDir.(*uio.BasicDirectory); ok {
			hamtDir, err := basicDir.SwitchToSharding(d.ctx)
			if err != nil {
				return err
			}
			d.unixfsDir = hamtDir
		}
	}

	err := d.unixfsDir.AddChild(d.ctx, name, node)
	if err != nil {
		return err
	}

	return nil
}

func (d *Directory) sync() error {
	for name, dir := range d.childDirs {
		//nd, err := dir.GetNode()
		// added by vingo
		if len(dir.AccessKey) <= 0 {
			dir.AccessKey = d.AccessKey
		}
 		nd, err := dir.GetNode()
 		dir.AccessKey = nil
 		//////////////////////
		if err != nil {
			return err
		}
		//  将 name 和 nd 更新到 d 的 unixfsDir中，需要对 nd 重新 encode 并计算 cid
		err = d.updateChild(name, nd)
		if err != nil {
			return err
		}
	}

	for name, file := range d.files {
		nd, err := file.GetNode()
		if err != nil {
			return err
		}

		err = d.updateChild(name, nd)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Directory) Path() string {
	cur := d
	var out string
	for cur != nil {
		switch parent := cur.parent.(type) {
		case *Directory:
			out = path.Join(cur.name, out)
			cur = parent
		case *Root:
			return "/" + out
		default:
			panic("directory parent neither a directory nor a root")
		}
	}
	return out
}

func (d *Directory) GetNode() (ipld.Node, error) {
	d.lock.Lock()
	defer d.lock.Unlock()

	err := d.sync()
	if err != nil {
		return nil, err
	}

	nd, err := d.unixfsDir.GetNode()
	if err != nil {
		return nil, err
	}
	// 在存储和缓存中寻找 nd，如果没找到则添加进去
	//err = d.dserv.Add(d.ctx, nd)
	//if err != nil {
	//	return nil, err
	//}
	// added by vingo
	//fmt.Println("======== encrypt ", d.name, "=======")
	if d.name != "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn" {
		n := nd.(*dag.ProtoNode)
		if len(n.AccessKey) <= 0 {
			n.AccessKey = d.AccessKey
		}
		err = d.dserv.Add(d.ctx, n)
	} else {
		err = d.dserv.Add(d.ctx, nd)
	}

	if err != nil {
		return nil, err
	}
	/////////////////

	nc := nd.Copy()

	return nc, err
}
