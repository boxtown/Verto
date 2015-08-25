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

	chain    *plugins
	compiled *plugins
}

// returns a fully initialized endpoint with handler
// as the http handler
func newEndpoint(method, path string, mux *PathMuxer, handler http.Handler) *endpoint {
	ep := &endpoint{
		method:  method,
		path:    path,
		mux:     mux,
		handler: handler,
		chain:   newPlugins(),
	}
	ep.compile()
	return ep
}

// compiles the chain of handlers for this endpoint
// with the passed in parentChain
func (ep *endpoint) compile() {
	ep.compiled = newPlugins()
	if ep.parent != nil {
		// parent exists so request copy from parent
		ep.compiled.link(ep.parent.compiled.deepCopy())
	} else if ep.mux != nil {
		// no parent so request copy from muxer
		ep.compiled.link(ep.mux.chain.deepCopy())
	}
	ep.compiled.link(ep.chain.deepCopy())
	ep.compiled.use(PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			ep.handler.ServeHTTP(w, r)
		},
	))
}

// Join sets a new group as parent and adjusts
// the endpoint's paths accordingly.
func (ep *endpoint) join(parent *group) {
	if ep.parent != nil {
		ep.parent.matcher.drop(ep.path)
	} else if ep.mux != nil {
		ep.mux.matchers[ep.method].drop(ep.path)
	}
	ep.parent = parent
	ep.path = trimPathPrefix(ep.path, parent.path, false)
	parent.matcher.add(ep.path, ep)
}

// ServeHTTP runs the compiled chain of handlers for this endpoint.
func (ep *endpoint) serveHTTP(w http.ResponseWriter, r *http.Request) {
	ep.compiled.run(w, r)
}

// Type returns the type of Compilable this is
func (ep *endpoint) cType() cType {
	return ENDPOINT
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
