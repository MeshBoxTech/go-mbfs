package libp2pquic

import (
	quic "mbfs/go-mbfs/gx/QmU44KWVkSHno7sNDTeUcL4FBgxgoidkFuTUyTXWJPXXFJ/quic-go"
	smux "mbfs/go-mbfs/gx/QmY9JXR3FupnYAYJWK9aMr9bCpqWKcToQ1tz8DVGTrHpHw/go-stream-muxer"
)

type stream struct {
	quic.Stream
}

var _ smux.Stream = &stream{}

func (s *stream) Reset() error {
	if err := s.Stream.CancelRead(0); err != nil {
		return err
	}
	return s.Stream.CancelWrite(0)
}
