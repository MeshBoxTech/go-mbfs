package main

import (
	plugin "mbfs/go-mbfs/gx/QmTzWwfHZr9N6Tnk4iDTtKMWY57D3wik73qNCVo21Bu1UP/iptb-plugins/browser"
	testbedi "mbfs/go-mbfs/gx/QmckeQ2zrYLAXoSHYTGn5BDdb22BqbUoHEHm8KZ9YWRxd1/iptb/testbed/interfaces"
)

var PluginName string
var NewNode testbedi.NewNodeFunc

func init() {
	PluginName = plugin.PluginName
	NewNode = plugin.NewNode
}
