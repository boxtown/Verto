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
	method string
	path   string

	parent  *group
	mux     *PathMuxer
	handler http.Handler

	chain    *Plugins
	compiled *Plugins
}

// returns a fully initialized endpoint with handler
// as the http handler
func newEndpoint(method, path string, mux *PathMuxer, handler http.Handler) *endpoint {
	ep := &endpoint{
		method:  method,
		path:    path,
		mux:     mux,
		handler: handler,
		chain:   NewPlugins(),
	}
	ep.Compile()
	return ep
}

// compiles the chain of handlers for this endpoint
// with the passed in parentChain
func (ep *endpoint) Compile() {
	ep.compiled = NewPlugins()
	if ep.parent != nil {
		// parent exists so request copy from parent
		ep.compiled.Link(ep.parent.compiled.DeepCopy())
	} else if ep.mux != nil {
		// no parent so request copy from muxer
		ep.compiled.Link(ep.mux.chain.DeepCopy())
	}
	ep.compiled.Link(ep.chain.DeepCopy())
	ep.compiled.Use(PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			ep.handler.ServeHTTP(w, r)
		},
	))
}

// Join sets a new group as parent and adjusts
// the endpoint's paths accordingly.
func (ep *endpoint) Join(parent *group) {
	if ep.parent != nil {
		ep.parent.matcher.Drop(ep.path)
	} else if ep.mux != nil {
		ep.mux.matchers[ep.method].Drop(ep.path)
	}
	ep.parent = parent
	ep.path = trimPathPrefix(ep.path, parent.path, false)
	parent.matcher.Add(ep.path, ep)
}

// ServeHTTP runs the compiled chain of handlers for this endpoint.
func (ep *endpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ep.compiled.Run(w, r)
}

// Type returns the type of Compilable this is
func (ep *endpoint) Type() CType {
	return ENDPOINT
}

// Use adds a PluginHandler onto the end of the chain of plugins
// for a node.
func (ep *endpoint) Use(handler PluginHandler) Endpoint {
	//ep.chain = append(ep.chain, handler)
	ep.chain.Use(handler)
	ep.Compile()
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
