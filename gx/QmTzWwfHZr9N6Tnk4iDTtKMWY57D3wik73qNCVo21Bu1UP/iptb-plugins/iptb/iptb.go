package main

import (
	"fmt"
	"os"

	cli "mbfs/go-mbfs/gx/QmckeQ2zrYLAXoSHYTGn5BDdb22BqbUoHEHm8KZ9YWRxd1/iptb/cli"
	testbed "mbfs/go-mbfs/gx/QmckeQ2zrYLAXoSHYTGn5BDdb22BqbUoHEHm8KZ9YWRxd1/iptb/testbed"

	browser "mbfs/go-mbfs/gx/QmTzWwfHZr9N6Tnk4iDTtKMWY57D3wik73qNCVo21Bu1UP/iptb-plugins/browser"
	docker "mbfs/go-mbfs/gx/QmTzWwfHZr9N6Tnk4iDTtKMWY57D3wik73qNCVo21Bu1UP/iptb-plugins/docker"
	local "mbfs/go-mbfs/gx/QmTzWwfHZr9N6Tnk4iDTtKMWY57D3wik73qNCVo21Bu1UP/iptb-plugins/local"
)

func init() {
	_, err := testbed.RegisterPlugin(testbed.IptbPlugin{
		From:        "<builtin>",
		NewNode:     local.NewNode,
		GetAttrList: local.GetAttrList,
		GetAttrDesc: local.GetAttrDesc,
		PluginName:  local.PluginName,
		BuiltIn:     true,
	}, false)

	if err != nil {
		panic(err)
	}

	_, err = testbed.RegisterPlugin(testbed.IptbPlugin{
		From:        "<builtin>",
		NewNode:     docker.NewNode,
		GetAttrList: docker.GetAttrList,
		GetAttrDesc: docker.GetAttrDesc,
		PluginName:  docker.PluginName,
		BuiltIn:     true,
	}, false)

	if err != nil {
		panic(err)
	}

	_, err = testbed.RegisterPlugin(testbed.IptbPlugin{
		From:       "<builtin>",
		NewNode:    browser.NewNode,
		PluginName: browser.PluginName,
		BuiltIn:    true,
	}, false)

	if err != nil {
		panic(err)
	}
}

func main() {
	cli := cli.NewCli()
	if err := cli.Run(os.Args); err != nil {
		fmt.Fprintf(cli.ErrWriter, "%s\n", err)
		os.Exit(1)
	}
}
