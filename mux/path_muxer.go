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
)

// ---------------------------------
// ---------- Path Muxer -----------

// PathMuxer matches string paths and methods to endpoint handlers.
// Paths can contain named parameters which can be restricted by regexes.
// PathMuxer also allows the use of global and per-route plugins.
type PathMuxer struct {
	parent  *PathMuxer
	chain   *plugins
	matcher Matcher
	prefix  string

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
		chain:   nil,
		matcher: &DefaultMatcher{},

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
	searchPath := replaceWildcards(path)
	results, err := mux.matcher.Match(searchPath)
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
		r.ParseForm()
		insertParams(params, r.Form)
	}

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
func (mux *PathMuxer) find(path string) (*muxNode, []Param, *plugins, error) {
	path = trimPathPrefix(path, mux.prefix, true)

	var params = make([]Param, 0, mux.matcher.MaxParams())
	var chain *plugins

	if mux.chain != nil && mux.chain.length > 0 {
		chain = mux.chain.deepCopy()
	}

	// Match subgroups
	result := mux.matcher.LongestPrefixMatch(path)
	cur, _ := result.Data().(*PathMuxer)
	prev := mux
	for cur != nil {
		if cur.chain != nil && cur.chain.length > 0 {
			if chain == nil {
				chain = cur.chain.deepCopy()
			} else {
				chain.link(cur.chain.deepCopy())
			}
		}

		// Handle wildcard path values
		if result.Params() != nil && len(result.Params()) > 0 {
			params = append(params, result.Params()...)
		}

		prev = cur
		path = trimPathPrefix(path, cur.prefix, true)
		result = cur.matcher.LongestPrefixMatch(path)
		cur, _ = result.Data().(*PathMuxer)
	}

	// It may be the case that we found the matching node
	// while matching subgroups
	if n, ok := isMatchingNode(result, path); ok {
		params = append(params, result.Params()...)
		return n, params, chain, nil
	}

	// Otherwise attempt a full match from the last found
	// subgroup or the initial mux
	var err error
	result, err = prev.matcher.Match(path)
	if err != nil {
		return nil, nil, nil, err
	}
	params = append(params, result.Params()...)
	return result.Data().(*muxNode), params, chain, nil
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

// Inserts parameters into a parameter map
func insertParams(params []Param, values url.Values) {
	if len(params) == 0 {
		return
	}
	for _, v := range params {
		values.Add(v.Key, v.Value)
	}
}

// Returns true if the results object contains a node
// with a path that matches the passed in path
func isMatchingNode(r Results, path string) (*muxNode, bool) {
	if n, ok := r.Data().(*muxNode); ok {
		if n.path == replaceWildcards(path) {
			return n, true
		}
		return nil, false
	}
	return nil, false
}

// Cleans a path by handling duplicate /'s,
// ., and ..
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

// If p does not end with a trailing slash, append one.
// Otherwise remove the trailing slash
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

// Replaces all wildcard designations in a path (/{...})
// with the wildcard character '*'
func replaceWildcards(path string) string {
	var buf bytes.Buffer
	for i := 0; i < len(path); i++ {
		r := path[i]
		if r != '{' || (r == '{' && i > 0 && path[i-1] != '/') {
			// Not possible start of wc, just write rune
			buf.WriteRune(rune(r))
		} else {
			// Possible start to wc
			for j := i + 1; j < len(path) && path[j] != '/'; j++ {
				if j == len(path)-1 && path[j] == '}' {
					// Found closing brace at end of path with no separator
					// inbetween. Write wc char and return replaced string
					buf.WriteRune('*')
					return buf.String()
				} else if j < len(path)-1 && path[j] == '}' && path[j+1] == '/' {
					// Found closing brace but more runes to write. Write
					// wc char and fast forward index.
					buf.WriteRune('*')
					i = j
					break
				} else if path[j] == '/' {
					// No closing brace found. Write original char and keep going.
					buf.WriteRune('{')
					break
				}
			}
		}
	}
	return buf.String()
}

// Trims a path prefix but counts wildcard segments (/{...})
// as equivalent and can optionally match against wildcards
// (skipWild: true means /{...} matches anything)
func trimPathPrefix(path, prefix string, skipWild bool) string {
	if len(prefix) >= len(path) {
		return path
	}

	i := 0
	j := 0
	m := 0
	n := 0
	for i < len(prefix) && j < len(path) {

		a := prefix[i]
		b := path[j]

		if a == '{' && (i == 0 || prefix[i-1] == '/') {
			// Possible start to prefix wc
			wildA := false
			for m = i + 1; m < len(prefix) && prefix[m] != '/'; m++ {
				if m == len(prefix)-1 && prefix[m] == '}' {
					// Closing brace found at end of prefix.
					wildA = true
					break
				} else if m < len(prefix)-1 && prefix[m] == '}' && prefix[m+1] == '/' {
					// Closing brace found with more runes to go
					wildA = true
					break
				}
			}
			if !wildA {
				if b == a {
					// No closing brace so no wild but b and a still match
					// so continue
					i++
					j++
					continue
				}
				// No wild and no a b match so break
				break
			}
			if b != '{' || (b == '{' && j > 0 && path[j-1] != '/') {
				// No possible start to path wc ergo no match
				if skipWild {
					// Skipping wilds so fast foward to next segment
					for ; i < len(prefix) && prefix[i] != '/'; i++ {
					}
					for ; j < len(path) && path[j] != '/'; j++ {
					}
					continue
				}
				// Not skipping wilds so no match ergo break here
				break
			} else {
				// Possible start to path wc
				wildB := false
				for n = j + 1; j < len(path) && path[n] != '/'; n++ {
					if n == len(path)-1 && path[n] == '}' {
						// Closing brace found at end of path
						wildB = true
						break
					} else if n < len(path)-1 && path[n] == '}' && path[n+1] == '/' {
						// Closing brace found with more runes to go
						wildB = true
						break
					}
				}
				if !wildB {
					// No brace ergo no path wc ergo no match
					if skipWild {
						// Skipping wild keep on rolling
						for ; i < len(prefix) && prefix[i] != '/'; i++ {
						}
						for ; j < len(path) && path[j] != '/'; j++ {
						}
						continue
					}
					// Not skipping break here
					break
				}
				i = m
				j = n
			}
		} else if a != b {
			break
		}
		i++
		j++
	}

	var buf bytes.Buffer
	for ; j < len(path); j++ {
		buf.WriteRune(rune(path[j]))
	}
	return buf.String()
}
