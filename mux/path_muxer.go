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
	chain     *plugins
	chainLock *sync.RWMutex
	nodes     Matcher
	groups    Matcher
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
		nodes:     &DefaultMatcher{},
		groups:    &DefaultMatcher{},

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
	searchPath := cleanWildcards(path)

	node, _, err := mux.findNode(searchPath)
	if err != nil {
		node = newMuxNode(mux, path)
		node.handlers[method] = handler
		mux.nodes.Add(path, node)
	} else {
		node.handlers[method] = handler
	}

	return newNodeImpl(method, node)
}

// AddFunc wraps f as an http.Handler and set is as handler for a specific method+path
// combination. AddFunc returns the endpoint node.
func (mux *PathMuxer) AddFunc(method, path string, f func(w http.ResponseWriter, r *http.Request)) Node {

	return mux.Add(method, path, http.Handler(http.HandlerFunc(f)))
}

func (mux *PathMuxer) Group(path string) Group {
	path = cleanPath(path)
	searchPath := cleanWildcards(path)

	group, _ := mux.findGroup(searchPath)
	if group != nil && cleanWildcards(group.prefix) == searchPath {
		return group
	} else if group != nil {
		path = trimPathPrefix(group.prefix, path, false)
		return group.Group(path)
	}

	group = New()
	group.prefix = path

	subNodes := mux.nodes.PrefixMatch(searchPath)
	subGroups := mux.groups.PrefixMatch(searchPath)

	for _, v := range subNodes {
		n := v.(*muxNode)
		mux.nodes.Delete(cleanWildcards(n.path))
		n.path = trimPathPrefix(n.path, path, false)
		group.nodes.Add(n.path, n)
	}
	for _, v := range subGroups {
		g := v.(*PathMuxer)
		mux.groups.Delete(cleanWildcards(g.prefix))
		g.prefix = trimPathPrefix(g.prefix, path, false)
		group.groups.Add(g.prefix, g)
	}

	mux.groups.Add(path, group)
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

	var node *muxNode = nil
	var values url.Values = url.Values{}
	var chain *plugins = nil
	var err error = nil

	if mux.chain != nil && mux.chain.length > 0 {
		mux.chainLock.RLock()
		chain = mux.chain.deepCopy()
		mux.chainLock.RUnlock()
	}

	// Match subgroups
	sub, vals := mux.findGroup(path)
	s := sub
	for s != nil {
		// Handle sub chains
		if s.chain != nil && s.chain.length > 0 {
			s.chainLock.RLock()
			if chain == nil {
				chain = s.chain.deepCopy()
			} else {
				chain.link(s.chain.deepCopy())
			}
			s.chainLock.RUnlock()
		}

		// Handle wildcard path values
		if vals != nil {
			for k, v := range vals {
				values[k] = v
			}
		}

		sub = s
		path = trimPathPrefix(path, sub.prefix, true)
		s, vals = sub.findGroup(path)
	}

	// Find endpoint node
	if sub != nil {
		node, vals, err = sub.findNode(path)
		if err != nil {
			return nil, nil, nil, err
		}
		if vals != nil {
			for k, v := range vals {
				values[k] = v
			}
		}
	} else {
		node, vals, err = mux.findNode(path)
		if err != nil {
			return nil, nil, nil, err
		}
		if vals != nil {
			for k, v := range vals {
				values[k] = v
			}
		}
	}
	return node, values, chain, nil
}

// findGroup returns the PathMuxer whose path is the longest
// prefix match of the passed in path or nil if no such
// muxer exists.
func (mux *PathMuxer) findGroup(path string) (*PathMuxer, url.Values) {
	results := mux.groups.LongestPrefixMatch(path)
	if results.Data() == nil {
		return nil, nil
	}
	pm, ok := results.Data().(*PathMuxer)
	if !ok {
		return nil, nil
	}
	return pm, results.Values()
}

// findNode finds and returns the node associated with the path
// plus any wildcard query parameters. Returns ErrNotFound if the
// path doesn't exist. Returns ErrRedirectSlash if a handler with (without)
// a trailing slash exists.
func (mux *PathMuxer) findNode(path string) (*muxNode, url.Values, error) {
	results, err := mux.nodes.Match(path)
	if err != nil {
		return nil, nil, err
	}
	node, _ := results.Data().(*muxNode)
	return node, results.Values(), nil
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

func cleanWildcards(path string) string {
	pathSplit := strings.Split(path, "/")
	for i := range pathSplit {
		if strings.HasPrefix(pathSplit[i], "{") && strings.HasSuffix(pathSplit[i], "}") {
			pathSplit[i] = wcStr
		}
	}

	if len(pathSplit) > 0 && len(pathSplit[0]) == 0 {
		pathSplit = pathSplit[1:]
	}

	searchPath := "/"
	for i := range pathSplit {
		segment := pathSplit[i]
		searchPath += segment

		if i < len(pathSplit)-1 {
			searchPath += "/"
		}
	}
	return searchPath
}

// Works the same as strings.TrimPrefix but treats
// wildcard path segments as equivalent.
func trimPathPrefix(path, prefix string, skipWild bool) string {
	if len(prefix) > len(path) {
		return path
	}

	var buf bytes.Buffer
	i := 0
	j := 0
	early := false

	for i < len(prefix) && !early {
		a := prefix[i]
		b := path[j]
		if a == '{' && a != b && skipWild {
			for a != '}' {
				i++
				if i == len(prefix) {
					early = true
					break
				}
				a = prefix[i]
			}
			if early {
				break
			}
			for b != '/' {
				j++
				if j == len(path) {
					early = true
					break
				}
				b = path[j]
			}
			if early {
				break
			}
			continue
		} else if a != b {
			break
		}

		if a == '{' {
			start := j
			for a != '}' {
				i++
				if i == len(prefix) {
					early = true
					j = start
					break
				}
				a = prefix[i]
			}
			if early {
				break
			}
			for b != '}' {
				j++
				if j == len(path) {
					early = true
					j = start
					break
				}
				b = path[j]
			}
			if early {
				break
			}
			i--
			j--
		}
		i++
		j++
	}

	for j < len(path) {
		buf.WriteRune(rune(path[j]))
		j++
	}

	return buf.String()
}
