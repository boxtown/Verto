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
	chain    *Plugins
	compiled *Plugins
	matchers map[string]*matcher

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
		chain:    NewPlugins(),
		matchers: make(map[string]*matcher),

		NotFound:       NotFoundHandler{},
		NotImplemented: NotImplementedHandler{},
		Redirect:       RedirectHandler{},

		Strict: true,
	}

	return &muxer
}

// Add sets the handler for a specific method+path combination
// and returns the endpoint node.
func (mux *PathMuxer) Add(method, path string, handler http.Handler) Endpoint {
	path = cleanPath(path)
	if strings.Contains(path, "/*/") {
		panic("PathMuxer.Add: '*' is reserved by PathMuxer.")
	}

	// Grab matcher for method
	m, ok := mux.matchers[method]
	if !ok {
		m = &matcher{}
		mux.matchers[method] = m
	}

	// Attempt to find pre-existing endpoint for path.
	// If it exists, set handler for endpoint. Otherwise
	// create new endpoint and add it to the muxer.
	var ep *endpoint
	results, err := m.matchNoRegex(path)
	if err != nil {
		ep = newEndpoint(method, path, mux, handler)
		ep.compile()
		m.add(path, ep)
	} else if results.data().cType() == GROUP {
		g := results.data().(*group)
		path = trimPathPrefix(path, g.path, false)
		return g.Add(path, handler)
	} else {
		ep = results.data().(*endpoint)
		ep.handler = handler
	}
	return ep
}

// AddFunc wraps f as an http.Handler and set is as handler for a specific method+path
// combination. AddFunc returns the endpoint node.
func (mux *PathMuxer) AddFunc(method, path string, f func(w http.ResponseWriter, r *http.Request)) Endpoint {
	return mux.Add(method, path, http.Handler(http.HandlerFunc(f)))
}

// Group creates a group at the passed in path.
// Groups and endpoints with paths that are
// subpaths of the passed in path are automatically
// subsumed by the newly created group.
// If there is a super-group that the passed in path
// falls under, the newly created group will be created
// under the super-group.
func (mux *PathMuxer) Group(method, path string) Group {
	path = cleanPath(path)

	// Drop path after/including catch-all
	if i := strings.Index(path, "^"); i != -1 {
		path = path[:i]
	}
	// Drop trailing slash as it doesn't make sense
	// in the context of groups
	if len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	// Root passed in, return current mux
	if len(path) == 0 || path == "/" {
		panic("PathMuxer.Group: Cannot group at mux root.")
	}

	// Check for equivalent or super groups.
	if c, _, _ := mux.find(method, path); c != nil {
		if c.cType() == GROUP {
			g := c.(*group)
			if pathsEqual(g.path, path) {
				return g
			} else {
				path = trimPathPrefix(path, g.path, false)
				return g.Group(path)
			}
		}
	}

	// Create new group
	g := newGroup(method, path, mux)

	// Gather subgroups, drop them from current mux/group,
	// add them to new group
	sub := make([]compilable, 0)
	m, ok := mux.matchers[method]
	if ok {
		m.applyAt(path, func(c compilable) {
			sub = append(sub, c)
		})
	} else {
		m = &matcher{}
		mux.matchers[method] = m
	}
	for _, c := range sub {
		c.join(g)
	}

	// Add group to current mux/group
	m.add(path, g)
	g.compile()
	return g
}

// Use adds a plugin handler onto the end of the chain of global
// plugins for the muxer.
func (mux *PathMuxer) Use(handler PluginHandler) *PathMuxer {
	//mux.chain = append(mux.chain, handler)
	mux.chain.Use(handler)
	for _, m := range mux.matchers {
		m.apply(func(c compilable) {
			c.compile()
		})
	}

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

// ServeHTTP dispatches the correct handler for the route.
func (mux *PathMuxer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p := cleanPath(r.URL.Path); p != r.URL.Path {
		r.URL.Path = p
		mux.Redirect.ServeHTTP(w, r)
		return
	}

	c, params, err := mux.find(r.Method, r.URL.Path)
	if err == ErrNotFound {
		mux.NotFound.ServeHTTP(w, r)
		return
	} else if err == ErrNotImplemented {
		mux.NotImplemented.ServeHTTP(w, r)
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
	c.serveHTTP(w, r)
}

// Find attempts to find the Compilable matching the passed in method+path
func (mux *PathMuxer) find(method, path string) (compilable, []param, error) {
	m, ok := mux.matchers[method]
	if !ok {
		return nil, nil, ErrNotImplemented
	}

	result, err := m.match(path)
	if err != nil {
		return nil, nil, err
	}
	return result.data(), result.params(), nil
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
func insertParams(params []param, values url.Values) {
	if len(params) == 0 {
		return
	}
	for _, v := range params {
		values.Add(v.key, v.value)
	}
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
		return p[:len(p)-1]
	}

	p += "/"
	return p
}

// checks if two paths are equal but
// counts wildcard segments (/{...}) as
// equivalent
func pathsEqual(p1, p2 string) bool {
	if len(p1) != len(p2) {
		return false
	}

	i := 0
	j := 0
	m := 0
	n := 0

	for i < len(p1) && j < len(p2) {
		a := p1[i]
		b := p2[j]

		if p1[i] == '{' && (i == 0 || p1[i-1] == '/') {
			// possible start to p1 wc
			wildA := false
			for m = i + 1; m < len(p1) && p1[m] != '/'; m++ {
				if m == len(p1)-1 && p1[m] == '}' {
					wildA = true
					break
				} else if m < len(p1)-1 && p1[m] == '}' && p1[m+1] == '/' {
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
				// No wild and no match so break
				return false
			}
			if b != '{' || (b == '{' && j > 0 && p2[j-1] != '/') {
				// No possible start to p2 wc ergo no match
				// so break here
				return false
			} else {
				// possible start to p2 wc
				wildB := false
				for n = j + 1; j < len(p2) && p2[n] != '/'; n++ {
					if n == len(p2)-1 && p2[n] == '}' {
						// Closing brace found at end of p2
						wildB = true
						break
					} else if n < len(p2)-1 && p2[n] == '}' && p2[n+1] == '/' {
						// Closing brace found with more runes to go
						wildB = true
						break
					}
				}
				if !wildB {
					// No brace ergo no p2 wc ergo no match
					// so break here
					return false
				}
				i = m
				j = n
			}
		} else if a != b {
			return false
		}
		i++
		j++
	}
	return true
}

// Trims a path prefix but counts wildcard segments (/{...})
// as equivalent. If the prefix cannot be found, no trimming
// is done. (skipWild: true means /{...} matches anything)
func trimPathPrefix(path, prefix string, skipWild bool) string {
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
				return path
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
				return path
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
					return path
				}
				i = m
				j = n
			}
		} else if a != b {
			return path
		}
		i++
		j++
	}

	if i < len(prefix) {
		return path
	}

	var buf bytes.Buffer
	for ; j < len(path); j++ {
		buf.WriteRune(rune(path[j]))
	}
	return buf.String()
}
