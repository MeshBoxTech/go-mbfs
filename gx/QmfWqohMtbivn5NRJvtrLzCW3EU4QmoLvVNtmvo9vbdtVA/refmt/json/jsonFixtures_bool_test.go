package json

import (
	"testing"

	"mbfs/go-mbfs/gx/QmfWqohMtbivn5NRJvtrLzCW3EU4QmoLvVNtmvo9vbdtVA/refmt/tok/fixtures"
)

func testBool(t *testing.T) {
	t.Run("bool true", func(t *testing.T) {
		checkCanonical(t, fixtures.SequenceMap["true"], `true`)
	})
	t.Run("bool false", func(t *testing.T) {
		checkCanonical(t, fixtures.SequenceMap["false"], `false`)
	})
}
