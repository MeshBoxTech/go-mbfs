package main

import (
	"io"
	"os"

	"mbfs/go-mbfs/gx/QmTsHcKgTQ4VeYZd8eKYpTXeLW7KNwkRD9wjnrwsV2sToq/go-colorable"
)

func main() {
	io.Copy(colorable.NewColorableStdout(), os.Stdin)
}
