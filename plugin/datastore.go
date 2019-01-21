package plugin

import (
	"mbfs/go-mbfs/repo/fsrepo"
)

// PluginDatastore is an interface that can be implemented to add handlers for
// for different datastores
type PluginDatastore interface {
	Plugin

	DatastoreTypeName() string
	DatastoreConfigParser() fsrepo.ConfigFromMap
}
