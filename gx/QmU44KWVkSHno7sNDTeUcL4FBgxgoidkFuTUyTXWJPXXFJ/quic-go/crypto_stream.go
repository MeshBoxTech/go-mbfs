package quic

import (
	"io"

	"mbfs/go-mbfs/gx/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go/internal/flowcontrol"
	"mbfs/go-mbfs/gx/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go/internal/protocol"
	"mbfs/go-mbfs/gx/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go/internal/wire"
)

type cryptoStream interface {
	StreamID() protocol.StreamID
	io.Reader
	io.Writer
	handleStreamFrame(*wire.StreamFrame) error
	popStreamFrame(protocol.ByteCount) (*wire.StreamFrame, bool)
	closeForShutdown(error)
	setReadOffset(protocol.ByteCount)
	// methods needed for flow control
	getWindowUpdate() protocol.ByteCount
	handleMaxStreamDataFrame(*wire.MaxStreamDataFrame)
}

type cryptoStreamImpl struct {
	*stream
}

var _ cryptoStream = &cryptoStreamImpl{}

func newCryptoStream(sender streamSender, flowController flowcontrol.StreamFlowController, version protocol.VersionNumber) cryptoStream {
	str := newStream(version.CryptoStreamID(), sender, flowController, version)
	return &cryptoStreamImpl{str}
}

// SetReadOffset sets the read offset.
// It is only needed for the crypto stream.
// It must not be called concurrently with any other stream methods, especially Read and Write.
func (s *cryptoStreamImpl) setReadOffset(offset protocol.ByteCount) {
	s.receiveStream.readOffset = offset
	s.receiveStream.frameQueue.readPos = offset
}
