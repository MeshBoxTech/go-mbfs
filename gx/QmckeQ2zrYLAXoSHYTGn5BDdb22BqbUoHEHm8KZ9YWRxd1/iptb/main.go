package main

import (
	"fmt"
	"os"

	"mbfs/go-mbfs/gx/QmckeQ2zrYLAXoSHYTGn5BDdb22BqbUoHEHm8KZ9YWRxd1/iptb/cli"
)

func main() {
	cli := cli.NewCli()

	if err := cli.Run(os.Args); err != nil {
		fmt.Fprintf(cli.ErrWriter, "%s\n", err)
		os.Exit(1)
	}
}
