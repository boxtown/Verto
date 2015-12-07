package recovery

import (
	"github.com/boxtown/verto"
	"github.com/boxtown/verto/plugins"
	"net/http"
)

// Recovery is a plugin that provides flexible, graceful panic recovery
// for web requests
type Recovery struct {
	// Core is the core functionality for plugins
	plugins.Core

	// OnRecover is the custom panic recovery function supplied by
	// the user. If OnRecover is nil, the plugin will just bubble the
	// panic up
	OnRecover func(rMsg interface{}, c *verto.Context)
}

// New instantiates and returns a new instance of a Recovery plugin
func New() *Recovery {
	return &Recovery{Core: plugins.Core{Id: "plugins.Recovery"}}
}

// Handle is called per web request to protect from program panics. If the OnRecover
// function is supplied on the plugin, OnRecover will be called to handle program
// panics. Otherwise, Handle will just bubble the panic up
func (plugin *Recovery) Handle(c *verto.Context, next http.HandlerFunc) {
	plugin.Core.Handle(
		func(c *verto.Context, next http.HandlerFunc) {
			r := c.Request
			w := c.Response
			next(w, r)
			if rMsg := recover(); rMsg != nil {
				if plugin.OnRecover != nil {
					plugin.OnRecover(rMsg, c)
				} else {
					panic(rMsg)
				}
			}
		}, c, next)
}
