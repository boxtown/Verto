package mux

import (
	"net/http"
	"strings"
)

// Group is an interface for interacting with route groups
type Group interface {
	// Add adds a handler to the passed in path under the group.
	// The full path to the handler will be the group's path concatenated
	// with the passed in path.
	Add(path string, handler http.Handler) Endpoint

	// AddFunc wraps f as an http.Handler and calls Add()
	AddFunc(path string, f func(w http.ResponseWriter, r *http.Request)) Endpoint

	// Group creates a subgroup at the passed in path. The full path for the new
	// subgroup is the parent group's path concatenated with the passed in path.
	// If a group already exists at the passed in path, the existing group is returned.
	// If a subgroup exists with a path that is a prefix of the passed in path, the new
	// subgroup will be created under the subgroup with the shorter prefix path. Any
	// existing paths that contain the passed in path as a prefix are subsumed under
	// the newly created subgroup.
	Group(path string) Group

	// Use appends handler on to the end of the Plugin chain for this group
	Use(handler PluginHandler) Group

	// UseHandler wraps handler as a PluginHandler and calls Use. Handler registered
	// using UseHandler automatically call the next-in-line Plugin.
	UseHandler(handler http.Handler) Group
}

// group implements the Group interface and the Compilable
// interface. The routing behavior mimics PathMuxer but only
// has one matcher as each group is method specific
type group struct {
	method   string
	path     string
	fullPath string

	parent  *group
	mux     *PathMuxer
	matcher Matcher

	chain    *Plugins
	compiled *Plugins
}

// newGroup returns a group with
// an empty initialized plugin chain
// and an initialized matcher
func newGroup(method, path string, mux *PathMuxer) *group {
	return &group{
		method:   method,
		path:     path,
		fullPath: path,
		mux:      mux,
		matcher:  &DefaultMatcher{},
		chain:    NewPlugins(),
		compiled: NewPlugins(),
	}
}

// Add adds a handler to the group at path. Wildcard characters
// are denoted by {}'s. A catch-all is denoted with ^. Segments
// after catch-alls are ignored. Wildcards may be further refined
// using regexes (e.g. {id: ^[0-9]$})
func (g *group) Add(path string, handler http.Handler) Endpoint {
	if strings.Contains(path, "/*/") {
		panic("PathMuxer.Add: '*' is reserved by PathMuxer.")
	}

	// Attempt to find pre-existing endpoint for path.
	// If it exists, set handler for endpoint. Otherwise
	// create new endpoint and add it to the muxer.
	var ep *endpoint
	results, err := g.matcher.MatchNoRegex(path)
	if err != nil {
		ep = newEndpoint(g.method, path, g.mux, handler)
		ep.parent = g
		ep.Compile()
		g.matcher.Add(path, ep)
	} else if results.Data().Type() == GROUP {
		g = results.Data().(*group)
		path = trimPathPrefix(path, g.path, false)
		return g.Add(path, handler)
	} else {
		ep = results.Data().(*endpoint)
		ep.handler = handler
	}
	return ep
}

// AddFunc wraps f as an http.Handler and calls g.Add()
func (g *group) AddFunc(path string, f func(w http.ResponseWriter, r *http.Request)) Endpoint {
	return g.Add(path, http.Handler(http.HandlerFunc(f)))
}

// Group creates a subgroup of the group at the passed
// in path. The subgroup's full path will be the path
// of the parent group plus the passed in path. Groups
// and endpoints with paths that are subpaths of the passed in path
// are automatically subsumed by the newly created group.
// If there is a super-subgroup that the passed in path
// falls under, the newly created group will be created
// under the super-subgroup.
func (g *group) Group(path string) Group {
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
		return g
	}

	// Check for equivalent or super groups.
	if c, _ := g.matcher.MatchNoRegex(path); c != nil {
		if c.Data().Type() == GROUP {
			ng := c.Data().(*group)
			if pathsEqual(ng.path, path) {
				return ng
			} else {
				path = trimPathPrefix(path, ng.path, false)
				return ng.Group(path)
			}
		}
	}

	// Create new group
	ng := newGroup(g.method, path, g.mux)

	// Gather subgroups, drop them from current mux/group,
	// add them to new group
	sub := make([]Compilable, 0)
	g.matcher.ApplyAt(path, func(c Compilable) {
		sub = append(sub, c)
	})
	for _, c := range sub {
		c.Join(ng)
	}

	// Add group to current mux/group
	ng.Join(g)
	ng.Compile()
	return ng
}

// Use adds a handler on to the chain of handlers
// for this group and then recompiles all chains
// in the subtree of group
func (g *group) Use(handler PluginHandler) Group {
	g.chain.Use(handler)
	g.Compile()
	return g
}

// UseHandler wraps handler as a PluginHandler and calls
// g.Use()
func (g *group) UseHandler(handler http.Handler) Group {
	pluginHandler := PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			handler.ServeHTTP(w, r)
			next(w, r)
		})

	g.Use(pluginHandler)
	return g
}

// Compile compiles the parent chain with
// the groups chain in order to avoid expensive
// chain manipulation during serving of requests.
// If the passed in chain is nil, then Compile will
// look towards the parent group or muxer for their
// compiled chains. Recompiles all chains in the
// subtree of group
func (g *group) Compile() {
	g.compiled = NewPlugins()
	if g.parent != nil {
		// parent exists so request copy from parent
		g.compiled.Link(g.parent.compiled.DeepCopy())
	} else if g.mux != nil {
		// no parent so must be top level group, request
		// copy from muxer
		g.compiled.Link(g.mux.chain.DeepCopy())
	}
	g.compiled.Link(g.chain.DeepCopy())
	g.matcher.Apply(func(c Compilable) {
		c.Compile()
	})
}

// Join sets a new group as parent and adjusts
// the group's paths accordingly.
func (g *group) Join(parent *group) {
	if g.parent != nil {
		g.parent.matcher.Drop(g.path)
	} else if g.mux != nil {
		g.mux.matchers[g.method].Drop(g.path)
	}
	g.parent = parent
	g.path = trimPathPrefix(g.path, parent.path, false)
	g.fullPath = parent.fullPath + g.path
	parent.matcher.Add(g.path, g)
}

// ServeHTTP attempts to find the correct endpoint for the request
// deferring to subgroups if need be. If the correct endpoint is found,
// the associated handler is run. Otherwise, the proper error response
// is returned.
func (g *group) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := trimPathPrefix(r.URL.Path, g.fullPath, true)
	if path[0] != '/' {
		path = "/" + path
	}

	result, err := g.matcher.Match(path)
	if err == ErrNotFound {
		g.mux.NotFound.ServeHTTP(w, r)
		return
	} else if err == ErrRedirectSlash {
		if !g.mux.Strict {
			r.URL.Path = handleTrailingSlash(r.URL.Path)
			g.mux.Redirect.ServeHTTP(w, r)
			return
		}
		g.mux.NotFound.ServeHTTP(w, r)
		return
	}

	if len(result.Params()) > 0 {
		r.ParseForm()
		insertParams(result.Params(), r.Form)
	}
	result.Data().ServeHTTP(w, r)
}

// Type returns the type of Compilable
// group is
func (g *group) Type() CType {
	return GROUP
}
