// Package tar provides functionality to write a unixfs merkledag
// as a tar archive.
package tar

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path"
	"time"

	ft "mbfs/go-mbfs/gx/QmXLCwhHh7bxRsBnCKNE9BAN87V44aSxXLquZYTtjr6fZ3/go-unixfs"
	uio "mbfs/go-mbfs/gx/QmXLCwhHh7bxRsBnCKNE9BAN87V44aSxXLquZYTtjr6fZ3/go-unixfs/io"
	mdag "mbfs/go-mbfs/gx/QmaDBne4KeY3UepeqSVKYpSmQGa3q9zP6x3LfVF2UjF3Hc/go-merkledag"

	ipld "mbfs/go-mbfs/gx/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"

	// added by vingo
	"github.com/ipfs/go-ipfs/core/crypto"
	"bytes"
)

// Writer is a utility structure that helps to write
// unixfs merkledag nodes as a tar archive format.
// It wraps any io.Writer.
type Writer struct {
	Dag  ipld.DAGService
	TarW *tar.Writer

	ctx context.Context
}

// NewWriter wraps given io.Writer.
func NewWriter(ctx context.Context, dag ipld.DAGService, w io.Writer) (*Writer, error) {
	return &Writer{
		Dag:  dag,
		TarW: tar.NewWriter(w),
		ctx:  ctx,
	}, nil
}

func (w *Writer) writeDir(nd *mdag.ProtoNode, fpath string) error {
	dir, err := uio.NewDirectoryFromNode(w.Dag, nd)
	if err != nil {
		return err
	}
	if err := writeDirHeader(w.TarW, fpath); err != nil {
		return err
	}

	return dir.ForEachLink(w.ctx, func(l *ipld.Link) error {
		child, err := w.Dag.Get(w.ctx, l.Cid)
		if err != nil {
			return err
		}
		npath := path.Join(fpath, l.Name)
		return w.WriteNode(child, npath)
	})
}

func (w *Writer) writeFile(nd *mdag.ProtoNode, fsNode *ft.FSNode, fpath string) error {
	if err := writeFileHeader(w.TarW, fpath, fsNode.FileSize()); err != nil {
		return err
	}

	// added by vingo
	if len(nd.AccessKey) > 0 {
		// get在这里解密目录下的小文件
		if len(nd.Links()) <= 0 {
			osize := len(fsNode.Data())

			// 对 data 解密
			origData := crypto.AesDecrypt(fsNode.Data(), nd.AccessKey)
			// 解密失败，可能是数据已经被破坏
			if origData == nil {
				return fmt.Errorf("Decrypt data error：%s", nd.String())
			}

			// 因为 CBC 模式加密时有补码的原因，解密后数据有可能变短了，需要在解密后的数据后面补码，
			// 使其和已经设置到 Writer 对象即 Response 头部的 Size 保持一致
			nsize := len(origData)
			//fmt.Println(osize,"----------",nsize)
			if osize-nsize > 0 {
				padtext := bytes.Repeat([]byte{byte(0)}, osize-nsize)
				origData = append(origData, padtext...)
			}

			// 将解密后的数据更新到 fsNode
			fsNode.SetData(origData)

		} else {
			// 设置解密大文件 Chunker 节点的密码，用于在 pbdagreader 中的 loadBufNode() 方法中读取 Chunker 节点数据时进行解密
			for _, lnk := range nd.Links() {
				if crypto.DecryptKey == nil {
					crypto.Init()
				}
				//fmt.Println(lnk.Cid.String(), "====Set link's acckey====", string(nd.AccessKey))
				crypto.DecryptKey[lnk.Cid.String()] = nd.AccessKey
			}
		}
	}
	///////////////////////

	dagr := uio.NewPBFileReader(w.ctx, nd, fsNode, w.Dag)
	if _, err := dagr.WriteTo(w.TarW); err != nil {
		return err
	}
	w.TarW.Flush()
	return nil
}

// WriteNode adds a node to the archive.
func (w *Writer) WriteNode(nd ipld.Node, fpath string) error {
	switch nd := nd.(type) {
	case *mdag.ProtoNode:
		fsNode, err := ft.FSNodeFromBytes(nd.Data())
		if err != nil {
			return err
		}
		switch fsNode.Type() {
		case ft.TMetadata:
			fallthrough
		case ft.TDirectory, ft.THAMTShard:
			return w.writeDir(nd, fpath)
		case ft.TRaw:
			fallthrough
		case ft.TFile:
			return w.writeFile(nd, fsNode, fpath)
		case ft.TSymlink:
			return writeSymlinkHeader(w.TarW, string(fsNode.Data()), fpath)
		default:
			return ft.ErrUnrecognizedType
		}
	case *mdag.RawNode:
		if err := writeFileHeader(w.TarW, fpath, uint64(len(nd.RawData()))); err != nil {
			return err
		}

		if _, err := w.TarW.Write(nd.RawData()); err != nil {
			return err
		}
		w.TarW.Flush()
		return nil
	default:
		return fmt.Errorf("nodes of type %T are not supported in unixfs", nd)
	}
}

// Close closes the tar writer.
func (w *Writer) Close() error {
	return w.TarW.Close()
}

func writeDirHeader(w *tar.Writer, fpath string) error {
	return w.WriteHeader(&tar.Header{
		Name:     fpath,
		Typeflag: tar.TypeDir,
		Mode:     0777,
		ModTime:  time.Now(),
		// TODO: set mode, dates, etc. when added to unixFS
	})
}

func writeFileHeader(w *tar.Writer, fpath string, size uint64) error {
	return w.WriteHeader(&tar.Header{
		Name:     fpath,
		Size:     int64(size),
		Typeflag: tar.TypeReg,
		Mode:     0644,
		ModTime:  time.Now(),
		// TODO: set mode, dates, etc. when added to unixFS
	})
}

func writeSymlinkHeader(w *tar.Writer, target, fpath string) error {
	return w.WriteHeader(&tar.Header{
		Name:     fpath,
		Linkname: target,
		Mode:     0777,
		Typeflag: tar.TypeSymlink,
	})
}
