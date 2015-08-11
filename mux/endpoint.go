package mux

import (
	"net/http"
)

// -------------------------------------
// ---------- Path Muxer Nodes ----------

// Endpoint is an interface for endpoints that
// allows for the addition of per-endpoint plugin
// handlers.
type Endpoint interface {
	// Use adds a PluginHandler onto the end of the chain of plugins
	// for an Endpoint.
	Use(handler PluginHandler) Endpoint

	// UseHandler wraps the handler as a PluginHandler and adds it onto the end
	// of the plugin chain.
	UseHandler(hander http.Handler) Endpoint
}

// muxNode is a private struct used to keep track of handlers
// and plugins per method+path.
type endpoint struct {
	handler http.Handler
	chain   *plugins
}

// ServeHTTP delegates to the appropriate handler based on
// the request method or calls the Not Implemented handler if
// the desired method handler does not exist. If there is a method-appropriate
// chain of plugins, those will be run first.
func (ep *endpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ep.chain == nil || ep.chain.length == 0 {
		ep.handler.ServeHTTP(w, r)
	} else {
		ep.chain.run(w, r)
	}
}

// Use adds a PluginHandler onto the end of the chain of plugins
// for a node.
func (ep *endpoint) Use(handler PluginHandler) Endpoint {
	if ep.chain == nil {
		ep.chain = newPlugins()
	}

	// Since we always add node.handler as the last handler,
	// we have to pop it off first before adding the desired handler.
	if ep.chain.length > 0 {
		ep.chain.popTail()
	}

	ep.chain.use(handler)
	ep.chain.use(PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			ep.handler.ServeHTTP(w, r)
		}))

	return ep
}

// UseHandler wraps the handler as a PluginHandler and adds it onto the end
// of the plugin chain.
func (ep *endpoint) UseHandler(handler http.Handler) Endpoint {
	pluginHandler := PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			handler.ServeHTTP(w, r)
			next(w, r)
		})

	ep.Use(pluginHandler)
	return ep
}
