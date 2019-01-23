package stream_test

import (
	. "mbfs/go-mbfs/gx/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo"
	. "mbfs/go-mbfs/gx/QmUWtNQd8JdEiYiDqNYTUcaqyteJZ2rTNQLiw3dauLPccy/gomega"

	"testing"
)

func TestGoLibp2pStreamTransport(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GoLibp2pStreamTransport Suite")
}
