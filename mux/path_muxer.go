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
	"strings"
	"sync"
)

// ---------------------------------
// ---------- Path Muxer -----------

// PathMuxer matches string paths and methods to endpoint handlers.
// Paths can contain named parameters which can be restricted by regexes.
// PathMuxer also allows the use of global and per-route plugins.
type PathMuxer struct {
	parent    *PathMuxer
	chain     *plugins
	chainLock *sync.RWMutex
	matcher   Matcher
	prefix    string

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

	return &muxer
}

// Add sets the handler for a specific method+path combination
// and returns the endpoint node.
func (mux *PathMuxer) Add(method, path string, handler http.Handler) Node {
	path = cleanPath(path)
	if strings.Contains(path, "/*/") {
		panic("PathMuxer.Add: '*' is reserved by PathMuxer.")
	}

	// Check for group that is prefix of desired path.
	// If one exists, recursively add handler to group.
	result := mux.matcher.LongestPrefixMatch(path)
	if group, ok := result.Data().(*PathMuxer); ok {
		path = trimPathPrefix(path, group.prefix, false)
		return group.Add(method, path, handler)
	}

	// No prefix group, attempt to find pre-existing
	// node for path. If it exists, set handler for node.
	// Otherwise create new node and add it to the muxer.
	var node *muxNode
	results, err := mux.matcher.Match(path)
	if err != nil {
		node = newMuxNode(mux, path)
		node.handlers[method] = handler
		mux.matcher.Add(path, node)
	} else {
		node = results.Data().(*muxNode)
		node.handlers[method] = handler
	}
	return newNodeImpl(method, node)
}

// AddFunc wraps f as an http.Handler and set is as handler for a specific method+path
// combination. AddFunc returns the endpoint node.
func (mux *PathMuxer) AddFunc(method, path string, f func(w http.ResponseWriter, r *http.Request)) Node {
	return mux.Add(method, path, http.Handler(http.HandlerFunc(f)))
}

// Group creates a route group at path. Any existing
// groups and nodes with a shared path prefix are subsumed.
// Attempting to create an existing group returns the existing
// group. Paths after and including catch all's are thrown
// away because catch all's are meaningless for groups.
func (mux *PathMuxer) Group(path string) Group {
	path = cleanPath(path)
	if path == "" || path == "/" {
		return mux
	}

	// Throw away anything after and including the catch all.
	if i := strings.Index(path, "^"); i != -1 {
		if i == len(path)-1 || path[i+1] == '/' {
			path = path[:i]
		}
	}
	searchPath := replaceWildcards(path)

	// Find matching or prefix group for path. If matching
	// group exists, return matching group. Otherwise if it's
	// a prefix group, recursively group under prefix group.
	result := mux.matcher.LongestPrefixMatch(searchPath)
	if group, ok := result.Data().(*PathMuxer); ok {
		if searchPath == replaceWildcards(group.prefix) {
			return group
		} else {
			path = trimPathPrefix(path, group.prefix, false)
			return group.Group(path)
		}
	}

	// No matching group or prefix group, create a new
	// group, grab all groups/nodes with path as a prefix
	// and delete them from current tree and add them to
	// new groups tree. Then add new group to muxer.
	group := New()
	group.parent = mux
	group.prefix = path
	subtree := mux.matcher.PrefixMatch(searchPath)
	mux.matcher.Drop(searchPath)
	for _, s := range subtree {
		if n, ok := s.(*muxNode); ok {
			n.path = trimPathPrefix(n.path, path, false)
			group.matcher.Add(n.path, n)
		} else {
			g := s.(*PathMuxer)
			g.prefix = trimPathPrefix(g.prefix, path, false)
			group.matcher.Add(g.prefix, g)
		}
	}
	mux.matcher.Add(path, group)
	return group
}

// Use adds a plugin handler onto the end of the chain of global
// plugins for the muxer.
func (mux *PathMuxer) Use(handler PluginHandler) Group {
	mux.chainLock.Lock()
	defer mux.chainLock.Unlock()

	if mux.chain == nil {
		mux.chain = newPlugins()
	}
	mux.chain.use(handler)

	return mux
}

// UseHandler wraps the handler as a PluginHandler and adds it onto the ned of
// the global plugin chain for the muxer.
func (mux *PathMuxer) UseHandler(handler http.Handler) Group {
	pluginHandler := PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			handler.ServeHTTP(w, r)
			next(w, r)
		})

	mux.Use(pluginHandler)
	return mux
}

// ServeHTTP dispatches the correct handler for the route.
func (mux *PathMuxer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p := cleanPath(r.URL.Path); p != r.URL.Path {
		r.URL.Path = p
		mux.Redirect.ServeHTTP(w, r)
		return
	}

	node, params, chain, err := mux.find(r.URL.Path)
	if err == ErrNotFound {
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

	mux.chainLock.RLock()
	defer mux.chainLock.RUnlock()

	if chain != nil {
		chain.use(PluginFunc(
			func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
				node.ServeHTTP(w, r)
			},
		))
		chain.run(w, r)
	} else {
		node.ServeHTTP(w, r)
	}
}

// find attempts to find the node and aggregate values and plugins for the node at
// path by first searching subgroups and then attempting to find the node
// directly. find returns an error if any is encountered while searching. For each
// subgroup found, find makes a copy of the plugin chain and links together these
// chains so that they run in the correct order. The average run time is
// O(m * n) where m is the average length of a plugin chain and n the average
// length of a path.
func (mux *PathMuxer) find(path string) (*muxNode, url.Values, *plugins, error) {
	path = trimPathPrefix(path, mux.prefix, true)

	var node *muxNode
	var values = url.Values{}
	var chain *plugins
	var err error

	mux.chainLock.RLock()
	if mux.chain != nil && mux.chain.length > 0 {
		chain = mux.chain.deepCopy()
	}
	mux.chainLock.RUnlock()

	// Match subgroups
	result := mux.matcher.LongestPrefixMatch(path)
	cur, _ := result.Data().(*PathMuxer)
	prev := cur
	for cur != nil {
		cur.chainLock.RLock()
		if cur.chain != nil && cur.chain.length > 0 {
			if chain == nil {
				chain = cur.chain.deepCopy()
			} else {
				chain.link(cur.chain.deepCopy())
			}
		}
		cur.chainLock.RUnlock()

		// Handle wildcard path values
		if result.Values() != nil {
			for k, v := range result.Values() {
				values[k] = v
			}
		}

		prev = cur
		path = trimPathPrefix(path, cur.prefix, true)
		result = cur.matcher.LongestPrefixMatch(path)
		cur, _ = result.Data().(*PathMuxer)
	}

	// Find endpoint node
	var end *PathMuxer
	if prev != nil {
		end = prev
	} else {
		end = mux
	}

	result, err = end.matcher.Match(path)
	if err != nil {
		return nil, nil, nil, err
	}
	if result.Values() != nil {
		for k, v := range result.Values() {
			values[k] = v
		}
	}
	node = result.Data().(*muxNode)
	return node, values, chain, nil
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

// RedirectHandler is the default http.Handler for Redirect responses. Returns a 301 status and redirects
// to the URL stored in r. This handler assumes the necessary adjustments to r.URL
// have been made prior to calling the handler.
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

func replaceWildcards(path string) string {
	pathSplit := strings.Split(path, "/")
	for i := range pathSplit {
		if isWild(pathSplit[i]) {
			pathSplit[i] = wcStr
		}
	}

	var buf bytes.Buffer
	for i := range pathSplit {
		segment := pathSplit[i]
		buf.WriteString(segment)

		if i < len(pathSplit)-1 {
			buf.WriteRune('/')
		}
	}
	return buf.String()
}

// Works the same as strings.TrimPrefix but treats
// wildcard path segments as equivalent.
func trimPathPrefix(path, prefix string, skipWild bool) string {
	pathSplit := strings.Split(path, "/")
	prefixSplit := strings.Split(prefix, "/")

	var i int
	for ; i < len(prefixSplit); i++ {
		a := prefixSplit[i]
		b := pathSplit[i]
		if isWild(a) && a != b && skipWild {
			continue
		} else if isWild(a) && isWild(b) {
			continue
		} else if a != b {
			break
		}
	}

	var buf bytes.Buffer
	if i == len(prefixSplit) && i != len(pathSplit) {
		buf.WriteRune('/')
	}
	for ; i < len(pathSplit); i++ {
		segment := pathSplit[i]
		buf.WriteString(segment)

		if i < len(pathSplit)-1 {
			buf.WriteRune('/')
		}
	}
	return buf.String()
}

func isWild(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}
