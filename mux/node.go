package mux

import (
	"net/http"
	"sync"
)

// -------------------------------------
// ---------- Path Muxer Nodes ----------

// Node is an interface for endpoint nodes that
// allows for the addition of per-route plugin
// handlers.
type Node interface {
	Use(handler PluginHandler) Node
	UseHandler(hander http.Handler) Node
}

// muxNode is the PathMuxer implementation of Node.
type muxNode struct {
	mux *PathMuxer

	chain      *plugins
	groupChain *plugins
	chainLock  *sync.RWMutex
	handlers   map[string]http.Handler

	path string
}

// newMuxNode returns a pointer to a newly initialized muxNode.
func newMuxNode(mux *PathMuxer, path string) *muxNode {
	return &muxNode{
		mux:        mux,
		chain:      newPlugins(),
		groupChain: newPlugins(),
		chainLock:  &sync.RWMutex{},
		handlers:   make(map[string]http.Handler),
		path:       path,
	}
}

// Use adds a PluginHandler onto the end of the chain of plugins
// for a node.
func (node *muxNode) Use(handler PluginHandler) Node {
	node.chainLock.Lock()
	defer node.chainLock.Unlock()

	if node.chain == nil {
		node.chain = newPlugins()
	}

	// Since we always add node.handler as the last handler,
	// we have to pop it off first before adding the desired handler.
	if node.chain.length > 0 {
		node.chain.popTail()
	}

	node.chain.use(handler)
	node.chain.use(PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			node.ServeHTTP(w, r)
		}))

	return node
}

// UseHandler wraps the handler as a PluginHandler and adds it onto the end
// of the plugin chain.
func (node *muxNode) UseHandler(handler http.Handler) Node {
	pluginHandler := PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			handler.ServeHTTP(w, r)
			next(w, r)
		})

	node.Use(pluginHandler)
	return node
}

// ServeHTTP delegates to the appropriate handler based on
// the request method or calls the Not Implemented handler if
// the desired method handler does not exist.
func (node *muxNode) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, ok := node.handlers[r.Method]
	if !ok {
		node.mux.NotImplemented.ServeHTTP(w, r)
		return
	}
	handler.ServeHTTP(w, r)
}
