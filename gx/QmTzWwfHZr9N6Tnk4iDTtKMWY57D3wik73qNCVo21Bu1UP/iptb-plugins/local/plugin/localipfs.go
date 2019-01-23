package main

import (
	plugin "mbfs/go-mbfs/gx/QmTzWwfHZr9N6Tnk4iDTtKMWY57D3wik73qNCVo21Bu1UP/iptb-plugins/local"
	testbedi "mbfs/go-mbfs/gx/QmckeQ2zrYLAXoSHYTGn5BDdb22BqbUoHEHm8KZ9YWRxd1/iptb/testbed/interfaces"
)

var PluginName string
var NewNode testbedi.NewNodeFunc
var GetAttrList testbedi.GetAttrListFunc
var GetAttrDesc testbedi.GetAttrDescFunc

func init() {
	PluginName = plugin.PluginName
	NewNode = plugin.NewNode
	GetAttrList = plugin.GetAttrList
	GetAttrDesc = plugin.GetAttrDesc
}
