package nested_test

import (
	"testing"

	. "github.com/onsi/gomega"
	. "mbfs/go-mbfs/gx/QmNuLxhqRhfimRZeLttPe6Sa44MNwuHAdaFFa9TDuNZUmf/ginkgo"
)

func TestNested(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nested Suite")
}
