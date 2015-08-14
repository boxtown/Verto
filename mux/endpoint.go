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

// endpoint is a private struct used to keep track of handlers
// and plugins per method+path.
type endpoint struct {
	handler  http.Handler
	groups   []*Group
	muxChain *plugins
	chain    *plugins
	compiled *plugins
}

// returns a fully initialized endpoint with handler
// as the http handler
func newEndpoint(handler http.Handler) *endpoint {
	ep := &endpoint{
		handler:  handler,
		muxChain: newPlugins(),
		chain:    newPlugins(),
	}
	ep.compile()
	return ep
}

// compiles the plugins of the mux, all groups,
// and the endpoint into one chain used for running.
// this function is called on every change to a chain.
func (ep *endpoint) compile() {
	ep.compiled = newPlugins()
	if ep.muxChain.length > 0 {
		ep.compiled.link(ep.muxChain.deepCopy())
	}
	if ep.groups != nil {
		for _, g := range ep.groups {
			ep.compiled.link(g.compiled.deepCopy())
		}
	}
	if ep.chain.length > 0 {
		ep.compiled.link(ep.chain.deepCopy())
	}
	ep.compiled.use(PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			ep.handler.ServeHTTP(w, r)
		},
	))
}

// ServeHTTP delegates to the appropriate handler based on
// the request method or calls the Not Implemented handler if
// the desired method handler does not exist. If there is a method-appropriate
// chain of plugins, those will be run first.
func (ep *endpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ep.compiled.run(w, r)
}

// Use adds a PluginHandler onto the end of the chain of plugins
// for a node.
func (ep *endpoint) Use(handler PluginHandler) Endpoint {
	//ep.chain = append(ep.chain, handler)
	ep.chain.use(handler)
	ep.compile()
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

	return ep.Use(pluginHandler)
}
