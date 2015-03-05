// Package mux provides a path multiplexer and
// interfaces for plugin handling and custom
// path matching.
package mux

// A custom multiplexer that can handle
// wildcards and regex routes.

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sync"
)

// -----------------------------
// ---------- Plugins ----------

// PluginHandler is an interface for Plugins.
// If the plugin ran successfully, call next
// to continue the chain of plugins.
type PluginHandler interface {
	Handle(w http.ResponseWriter, r *http.Request, next http.HandlerFunc)
}

// PluginFunc wraps functions so that they implement the PluginHandler interface.
type PluginFunc func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc)

// Handle calls the function wrapped as a PluginFunc.
func (p PluginFunc) Handle(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	p(w, r, next)
}

// Plugin implements the http.Handler interface. It is a linked list
// of plugins.
type plugin struct {
	handler PluginHandler
	next    *plugin
	prev    *plugin
}

var emptyPlugin = &plugin{
	handler: PluginFunc(func(w http.ResponseWriter, r *http.Request, nex http.HandlerFunc) {

	}),
}

func (p *plugin) run(w http.ResponseWriter, r *http.Request) {
	p.handler.Handle(w, r, p.next.run)
}

type plugins struct {
	head *plugin
	tail *plugin

	length int64
}

func newPlugins() *plugins {
	return &plugins{emptyPlugin, emptyPlugin, 0}
}

func (p *plugins) use(handler PluginHandler) {
	p.length = p.length + 1

	plugin := &plugin{
		handler: handler,
		next:    emptyPlugin,
		prev:    emptyPlugin,
	}

	if p.head == nil || p.head == emptyPlugin {
		p.head = plugin
		p.tail = p.head
		return
	}

	p.tail.next = plugin
	plugin.prev = p.tail
	p.tail = plugin
}

func (p *plugins) popHead() {
	if p.head == nil || p.head == emptyPlugin {
		return
	}

	p.length = p.length - 1

	p.head = p.head.next
	if p.head != nil && p.head != emptyPlugin {
		p.head.prev = emptyPlugin
	} else {
		p.tail = emptyPlugin
	}
}

func (p *plugins) popTail() {
	if p.tail == nil || p.tail == emptyPlugin {
		return
	}

	p.length = p.length - 1

	p.tail = p.tail.prev
	if p.tail != nil && p.tail != emptyPlugin {
		p.tail.next = emptyPlugin
	} else {
		p.head = emptyPlugin
	}
}

func (p *plugins) run(w http.ResponseWriter, r *http.Request) {
	if p.head == nil || p.head == emptyPlugin {
		return
	}

	p.head.run(w, r)
}

// -------------------------------------
// ---------- Path Tree Nodes ----------

// Node is an interface for endpoint nodes that
// allows for the addition of per-route plugin
// handlers.
type Node interface {
	Use(handler PluginHandler) Node
	UseHandler(hander http.Handler) Node
}

// MuxNode is the PathMuxer implementation of Node.
type MuxNode struct {
	chain     *plugins
	chainLock *sync.RWMutex
	handler   http.Handler
}

// NewMuxNode returns a pointer to a newly initialized MuxNode.
func NewMuxNode() *MuxNode {
	return &MuxNode{
		chain:     newPlugins(),
		chainLock: &sync.RWMutex{},
	}
}

// Use adds a PluginHandler onto the end of the chain of plugins
// for a node.
func (node *MuxNode) Use(handler PluginHandler) Node {
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
			if node.handler == nil {
				return
			}
			node.handler.ServeHTTP(w, r)
		}))

	return node
}

// UseHandler wraps the handler as a PluginHandler and adds it onto the end
// of the plugin chain.
func (node *MuxNode) UseHandler(handler http.Handler) Node {
	pluginHandler := PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			handler.ServeHTTP(w, r)
			next(w, r)
		})

	node.Use(pluginHandler)
	return node
}

// ---------------------------------
// ---------- Path Muxer -----------

// PathMuxer matches string paths and methods to endpoint handlers.
// Paths can contain named parameters which can be restricted by regexes.
// PathMuxer also allows the use of global and per-route plugins.
type PathMuxer struct {
	chain     *plugins
	chainLock *sync.RWMutex
	matcher   Matcher

	NotFound       http.Handler
	NotImplemented http.Handler
	Redirect       http.Handler

	// If strict, Paths with trailing slashes are considered
	// a different path than those without trailing slashes.
	// E.g. '/a/b/' != '/a/b'.
	Strict bool
}

// New returns a pointer to a newly initialized PathMuxer.
func New() *PathMuxer {
	muxer := PathMuxer{
		chain:     newPlugins(),
		chainLock: &sync.RWMutex{},
		matcher:   &DefaultMatcher{},

		NotFound:       NotFoundHandler{},
		NotImplemented: NotImplementedHandler{},
		Redirect:       RedirectHandler{},

		Strict: true,
	}

	muxer.chain.use(PluginFunc(muxer.endpoint))

	return &muxer
}

func (mux *PathMuxer) endpoint(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	node, params, err := mux.find(r.Method, r.URL.Path)
	if err == ErrNotImplemented {
		mux.NotImplemented.ServeHTTP(w, r)
		return
	} else if err == ErrNotFound {
		mux.NotFound.ServeHTTP(w, r)
		return
	} else if err == ErrRedirectSlash {
		if !mux.Strict {
			r.URL.Path = handleTrailingSlash(r.URL.Path)
			mux.Redirect.ServeHTTP(w, r)
			return
		}

		mux.NotFound.ServeHTTP(w, r)
		return
	}

	if len(params) > 0 {
		r.URL.RawQuery = appendParams(r.URL.RawQuery, params.Encode())
	}

	node.chainLock.RLock()
	defer node.chainLock.RUnlock()

	if node.chain == nil || node.chain.length == 0 {
		node.handler.ServeHTTP(w, r)
	} else {
		node.chain.run(w, r)
	}
}

// Use adds a plugin handler onto the end of the chain of global
// plugins for the muxer.
func (mux *PathMuxer) Use(handler PluginHandler) *PathMuxer {
	mux.chainLock.Lock()
	defer mux.chainLock.Unlock()

	if mux.chain == nil {
		mux.chain = newPlugins()
	}

	tail := mux.chain.tail
	if tail != nil {
		mux.chain.popTail()
	}

	mux.chain.use(handler)
	mux.chain.use(tail.handler)
	return mux
}

// UseHandler wraps the handler as a PluginHandler and adds it onto the ned of
// the global plugin chain for the muxer.
func (mux *PathMuxer) UseHandler(handler http.Handler) *PathMuxer {
	pluginHandler := PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			handler.ServeHTTP(w, r)
			next(w, r)
		})

	mux.Use(pluginHandler)
	return mux
}

// Add sets the handler for a specific method+path combination
// and returns the endpoint node.
func (mux *PathMuxer) Add(method, path string, handler http.Handler) Node {
	path = cleanPath(path)

	node, _, err := mux.find(method, path)
	if err != nil {
		node = NewMuxNode()
		node.handler = handler
		mux.matcher.Add(method, path, node)
	} else {
		node.handler = handler
	}

	return node
}

// AddFunc wraps f as an http.Handler and set is as handler for a specific method+path
// combination. AddFunc returns the endpoint node.
func (mux *PathMuxer) AddFunc(method, path string, f func(w http.ResponseWriter, r *http.Request)) Node {

	return mux.Add(method, path, http.Handler(http.HandlerFunc(f)))
}

// Finds and returns the handler associated with the method+path
// plus any wildcard query parameters. Returns ErrNotFound if the
// path doesn't exist or ErrNotImplemented if there is no handler
// for that method+path. Returns ErrRedirectSlash if a handler with (without)
// a trailing slash exists.
func (mux *PathMuxer) find(method, path string) (*MuxNode, url.Values, error) {
	path = cleanPath(path)

	data, values, err := mux.matcher.Match(method, path)
	if err != nil {
		return nil, nil, err
	}
	node, ok := data.(*MuxNode)
	if !ok {
		return nil, nil, ErrNotFound
	}
	return node, values, nil
}

// ServeHTTP dispatches the correct handler for the route.
func (mux *PathMuxer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p := cleanPath(r.URL.Path); p != r.URL.Path {
		r.URL.Path = p
		mux.Redirect.ServeHTTP(w, r)
		return
	}

	mux.chainLock.RLock()
	defer mux.chainLock.RUnlock()

	mux.chain.run(w, r)
}

// -----------------------------
// ---------- Helpers ----------

// NotFoundHandler is the default http.Handler for Not Found responses. Returns a 404 status
// with message "Not Found."
type NotFoundHandler struct{}

func (handler NotFoundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "Not Found.")
}

// NotImplementedHandler is the default http.Handler for Not Implemented responses. Returns a 501 status
// with message "Not Implemented."
type NotImplementedHandler struct{}

func (handler NotImplementedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	fmt.Fprintf(w, "Not Implemented.")
}

// ReirectHandler is the default http.Handler for Redirect responses. Returns a 301 status and redirects
// to the URL stored in r. This handler assumes the necessary adjustments to r.URL
// have been made prior to calling the handler.
type RedirectHandler struct{}

func (handler RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusMovedPermanently)
	w.Header().Set("Location", r.URL.String())
}

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}
	return np
}

func handleTrailingSlash(p string) string {
	if p == "" {
		return "/"
	}

	if p[len(p)-1] == '/' {
		b := []byte(p)
		b = b[:len(b)-1]
		return string(b)
	}

	p += "/"
	return p
}

func appendParams(query string, params string) string {
	var buf bytes.Buffer
	buf.WriteString(query)
	buf.WriteString("&")
	buf.WriteString(params)
	return buf.String()
}
