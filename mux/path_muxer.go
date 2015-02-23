// PathMuxer
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

// An interface for Plugins
type PluginHandler interface {
	Handle(w http.ResponseWriter, r *http.Request, next http.HandlerFunc)
}

// PluginFunc is a function that implements the PluginHandler interface.
type PluginFunc func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc)

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

func (p *plugin) run(w http.ResponseWriter, r *http.Request) {
	p.handler.Handle(w, r, p.next.run)
}

type plugins struct {
	head *plugin
	tail *plugin

	length int64
}

func (p *plugins) use(handler PluginHandler) {
	p.length = p.length + 1

	plugin := &plugin{
		handler: handler,
		next: &plugin{
			handler: PluginFunc(func(
				w http.ResponseWriter,
				r *http.Request,
				next http.HandlerFunc) {
			}),
		},
	}

	if p.head == nil {
		p.head = plugin
		p.tail = p.head
		return
	}

	p.tail.next = plugin
	plugin.prev = p.tail
	p.tail = plugin
}

func (p *plugins) popHead() {
	if p.head == nil {
		return
	}

	p.length = p.length - 1

	p.head = p.head.next
	if p.head != nil {
		p.head.prev = nil
	} else {
		p.tail = nil
	}
}

func (p *plugins) popTail() {
	if p.tail == nil {
		return
	}

	p.length = p.length - 1

	p.tail = p.tail.prev
	if p.tail != nil {
		p.tail.next = nil
	} else {
		p.head = nil
	}
}

func (p *plugins) run(w http.ResponseWriter, r *http.Request) {
	if p.head == nil {
		return
	}

	p.head.run(w, r)
}

// -------------------------------------
// ---------- Path Tree Nodes ----------

type Node interface {
	Use(handler PluginHandler) Node
	UseHandler(hander http.Handler) Node
}

type MuxNode struct {
	chain     *plugins
	chainLock *sync.RWMutex
	handler   http.Handler
}

func NewMuxNode() *MuxNode {
	return &MuxNode{
		chain:     &plugins{},
		chainLock: &sync.RWMutex{},
	}
}

// Adds a PluginHandler onto the end of the chain of plugins
// for this node.
func (node *MuxNode) Use(handler PluginHandler) Node {
	node.chainLock.Lock()
	defer node.chainLock.Unlock()

	if node.chain == nil {
		node.chain = &plugins{}
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

// Wrap the handler as a PluginHandler and add it onto the end
// of the plguin chain for this node.
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

// Returns a pointer to a new, empty PathMuxer.
func New() *PathMuxer {
	muxer := PathMuxer{
		chain:     &plugins{},
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
	} else if err == ErrRedirectSlash && !mux.Strict {
		mux.Redirect.ServeHTTP(w, r)
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

// Add a plugin handler onto the end of the chain of global
// plugins for the muxer.
func (mux *PathMuxer) Use(handler PluginHandler) *PathMuxer {
	mux.chainLock.Lock()
	defer mux.chainLock.Unlock()

	if mux.chain == nil {
		mux.chain = &plugins{}
	}

	tail := mux.chain.tail
	if tail != nil {
		mux.chain.popTail()
	}

	mux.chain.use(handler)
	mux.chain.use(tail.handler)
	return mux
}

// Wrap the handler as a PluginHandler and add it onto the ned of
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

// Sets the handler for a specific method+path combination. Can handle
// wildcard paths e.g. /account/{id}. Returns the endpoint node.
func (mux *PathMuxer) Add(method, path string, handler http.Handler) Node {
	path = cleanPath(path)

	node, _, err := mux.find(method, path)
	if err != nil {
		node = NewMuxNode()
		node.handler = handler
		mux.matcher.Add(method, path, node)
	}

	return node
}

// Sets a handler with handler function f for a specifi method+path
// combination. Can handle wildcard paths e.g. /account/{id}. Returns the endpoint node.
func (mux *PathMuxer) AddFunc(method, path string, f func(w http.ResponseWriter, r *http.Request)) Node {

	return mux.Add(method, path, http.Handler(http.HandlerFunc(f)))
}

// Finds and returns the handler associated with the method+path
// plus any wildcard query parameters. Returns ErrNotFound if the
// path doesn't exist or ErrNotImplemented if there is no handler
// for that method+path.
func (mux *PathMuxer) find(method, path string) (*MuxNode, url.Values, error) {
	path = cleanPath(path)

	data, values, err := mux.matcher.Match(method, path)
	if node, ok := data.(*MuxNode); ok {
		return node, values, err
	} else {
		return nil, nil, ErrNotFound
	}
}

// ServeHTTP dispatches the correct handler for the route.
func (mux *PathMuxer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p := cleanPath(r.URL.Path); p != r.URL.Path {

		url := *r.URL
		url.Path = p

		mux.Redirect.ServeHTTP(w, r)
		return
	}

	mux.chainLock.RLock()
	defer mux.chainLock.RUnlock()

	mux.chain.run(w, r)
}

// -----------------------------
// ---------- Helpers ----------

type NotFoundHandler struct{}

func (handler NotFoundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(404)
	fmt.Fprintf(w, "Not Found.")
}

type NotImplementedHandler struct{}

func (handler NotImplementedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(501)
	fmt.Fprintf(w, "Not Implemented.")
}

type RedirectHandler struct{}

func (handler RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", r.URL.String())
	w.WriteHeader(http.StatusMovedPermanently)
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
