// Package mux provides a path multiplexer and
// interfaces for plugin handling and custom
// path matching.
package mux

// A custom multiplexer that can handle
// wildcards and regex routes.

import (
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
	parent   *PathMuxer
	chain    *plugins
	matchers map[string]Matcher

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
		chain:    nil,
		matchers: make(map[string]Matcher),

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
		m = &DefaultMatcher{}
		mux.matchers[method] = m
	}

	// Attempt to find pre-existing endpoint for path.
	// If it exists, set handler for endpoint. Otherwise
	// create new endpoint and add it to the muxer.
	var ep *endpoint
	results, err := m.MatchNoRegex(path)
	if err != nil {
		ep = &endpoint{handler: handler}
		m.Add(path, ep)
	} else {
		ep = results.Data().(*endpoint)
		ep.handler = handler
	}
	return ep
}

// AddFunc wraps f as an http.Handler and set is as handler for a specific method+path
// combination. AddFunc returns the endpoint node.
func (mux *PathMuxer) AddFunc(method, path string, f func(w http.ResponseWriter, r *http.Request)) Endpoint {
	return mux.Add(method, path, http.Handler(http.HandlerFunc(f)))
}

// Use adds a plugin handler onto the end of the chain of global
// plugins for the muxer.
func (mux *PathMuxer) Use(handler PluginHandler) *PathMuxer {
	if mux.chain == nil {
		mux.chain = newPlugins()
	}
	mux.chain.use(handler)

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

	ep, params, err := mux.find(r.Method, r.URL.Path)
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

	if mux.chain != nil && mux.chain.length > 0 {
		chain := mux.chain.deepCopy()
		chain.use(PluginFunc(
			func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
				ep.ServeHTTP(w, r)
			},
		))
		chain.run(w, r)
	} else {
		ep.ServeHTTP(w, r)
	}
}

// Find attempts to find the endpoint matching the passed in method+path
func (mux *PathMuxer) find(method, path string) (*endpoint, []Param, error) {
	m, ok := mux.matchers[method]
	if !ok {
		return nil, nil, ErrNotImplemented
	}

	result, err := m.Match(path)
	if err != nil {
		return nil, nil, err
	}
	return result.Data().(*endpoint), result.Params(), nil
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
