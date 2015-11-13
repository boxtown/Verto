// Package verto is a simple REST framework. It is
// plug n' play and includes it's own path
// multiplexer, error handler, and response
// handler. It is recommended to bring your
// own error handling and response handling.
// Verto provides users the option to use
// middleware globally or per route. The
// Verto multiplexer is not substitutable
package verto

import (
	"fmt"
	"github.com/boxtown/verto/mux"
	"net"
	"net/http"
)

// -------------------------------------------
// -------- Interfaces/Definitions -----------

// ErrorHandler is the Verto-specific interface for error handlers.
// A default ErrorHandler is provided with Verto but it is recommended
// to bring your own ErrorHandler.
type ErrorHandler interface {

	// Handle handles the error. Context is guaranteed to be
	// populated if ErrorHandler is registed through Verto.
	Handle(err error, c *Context)
}

// ErrorFunc wraps functions so that they implement ErrorHandler
type ErrorFunc func(err error, c *Context)

// Handle calls the function wrapped by ErrorFunc.
func (erf ErrorFunc) Handle(err error, c *Context) {
	erf(err, c)
}

// ResponseHandler is the Verto-specific interface for response handlers.
// A default ResponseHandler is provided with Verto but it is recommended
// to bring your own ResponseHandler.
type ResponseHandler interface {

	// Handle handles the response. Context is guaranteed to be
	// populated if ResponseHandler is registered through Verto.
	Handle(response interface{}, c *Context)
}

// ResponseFunc wraps functions so that they implement ResponseHandler
type ResponseFunc func(response interface{}, c *Context)

// Handle calls the function wrapped by ResponseFunc.
func (rf ResponseFunc) Handle(response interface{}, c *Context) {
	rf(response, c)
}

// --------------------------------
// ----------- Wrappers -----------

// Plugin is a custom plugin definition for Verto that allows injections by
// context.
type Plugin interface {
	Handle(c *Context, next http.HandlerFunc)
}

// PluginFunc wraps functions as Verto Plugins
type PluginFunc func(c *Context, next http.HandlerFunc)

// Handle calls functions wrapped by VertoPluginFunc.
func (pf PluginFunc) Handle(c *Context, next http.HandlerFunc) {
	pf(c, next)
}

// Endpoint is an object returned by add route functions
// that allow the addition of plugins to be executed on the
// added route. Endpoint is able to handle plain http.Handlers,
// mux.PluginHandlers, and verto.Plugins as middleware plugins.
// Endpoint is a wrapper around mux.Endpoint
type Endpoint struct {
	mux.Endpoint
	v *Verto
}

// Use adds a mux.PluginHandler onto the chain of plugins to be executed
// when the route represented by the Endpoint is requested.
func (ep *Endpoint) Use(handler mux.PluginHandler) *Endpoint {
	return &Endpoint{ep.Endpoint.Use(handler), ep.v}
}

// UseHandler adds an http.handler onto the chain of plugins to be
// executed when the route represented by the Endpoint is requested.
// http.Handler plugins will always call the next-in-line plugin if
// one exists
func (ep *Endpoint) UseHandler(handler http.Handler) *Endpoint {
	return &Endpoint{ep.Endpoint.UseHandler(handler), ep.v}
}

// UseVerto adds a Plugin onto the chain of plugins to be
// executed when the route represented by the Endpoint is requested.
// The Plugin will have it's context provided by the Verto instance
// that generated the Endpoint
func (ep *Endpoint) UseVerto(plugin Plugin) *Endpoint {
	pluginFunc := func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		c := NewContext(w, r, ep.v.Injections, ep.v.Logger)
		plugin.Handle(c, next)
	}
	return &Endpoint{ep.Endpoint.Use(mux.PluginFunc(pluginFunc)), ep.v}
}

// Group represents a group of routes in Verto. Routes are generally
// grouped by a shared path prefix but can also be grouped by method
// as well. Group allows the addition of plugins to be run whenever
// a path within the group is requested
type Group struct {
	g mux.Group
	v *Verto
}

// Add registers a ResourceFunc at the path under Group. The resulting
// route will have a full path equivalent to the passed in path appended
// onto the Group's path prefix. An Endpoint representing the added route
// is returned. If the path already exists, this function will overwrite the
// old handler with the passed in ResourceFunc.
func (g *Group) Add(path string, rf ResourceFunc) *Endpoint {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		c := NewContext(w, r, g.v.Injections, g.v.Logger)
		response, err := rf(c)
		if err != nil {
			g.v.ErrorHandler.Handle(err, c)
		} else {
			g.v.ResponseHandler.Handle(response, c)
		}
	}
	return &Endpoint{g.g.AddFunc(path, handlerFunc), g.v}
}

// AddHandler registers an http.Handler as the handler for the passed in path.
// AddHandler behaves exactly the same as Add except that it takes in an http.Handler
// instead of a ResourceFunc
func (g *Group) AddHandler(path string, handler http.Handler) *Endpoint {
	return &Endpoint{g.g.Add(path, handler), g.v}
}

// Group registers a sub-Group under the current Group at the
// passed in path. The new Group's full path is equivalent to
// the passed in path appended to the current Group's path prefix.
// Any existing endpoints and groups who might fall under the new Group
// (e.g. path prefix == new Group's path) will be subsumed by the new Group.
// If a sub-Group exists with a path that is a path prefix of the would-be new
// Group, the new Group is added under the sub-Group instead. If a sub-Group already
// exists at the given path, the existing Group is not overwritten and is returned.
// Otherwise the newly created Group is returned.
func (g *Group) Group(path string) *Group {
	return &Group{g.g.Group(path), g.v}
}

// Use adds a mux.PluginHandler as a plugin to be executed for all
// paths and sub-Groups under the current group.
func (g *Group) Use(handler mux.PluginHandler) *Group {
	return &Group{g.g.Use(handler), g.v}
}

// UseHandler adds an http.Handler as a plugin to be executed for all
// paths and sub-Groups under the current Group. http.Handler plugins
// will always call the next-in-line plugin if one exists
func (g *Group) UseHandler(handler http.Handler) *Group {
	return &Group{g.g.UseHandler(handler), g.v}
}

// UseVerto adds a Plugin to be executed for all paths and sub-Groups
// under the current group.
func (g *Group) UseVerto(plugin Plugin) *Group {
	pluginFunc := func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		c := NewContext(w, r, g.v.Injections, g.v.Logger)
		plugin.Handle(c, next)
	}
	return &Group{g.g.Use(mux.PluginFunc(pluginFunc)), g.v}
}

// ResourceFunc is the Verto-specific function for endpoint resource handling.
type ResourceFunc func(c *Context) (interface{}, error)

// ----------------------------
// ---------- Verto -----------

// Verto is a simple and fast REST framework. It has a simple to use but powerful
// API that allows you to quickly create RESTful Go backends.
//
// Example usage:
//	// Instantiates a new Verto instance and registers a hello world handler
// 	// at GET /hello/world
//	v := verto.New()
//	v.Get("/hello/world", verto.ResourceFunc(func(c *verto.Context) {
//		return "Hello, World!"
//	}))
type Verto struct {
	Injections      *Injections
	Logger          Logger
	ErrorHandler    ErrorHandler
	ResponseHandler ResponseHandler

	verbose bool
	sl      *StoppableListener
	muxer   *mux.PathMuxer
}

// HttpHandler is a wrapper around Verto such that it can run
// as an http.handler
type HttpHandler struct {
	*Verto
}

// ServeHTTP serves requests directly to Verto's muxer.
func (handler *HttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.muxer.ServeHTTP(w, r)
}

// New returns a newly initialized Verto instance.
// The path /shutdown is automatically reserved as a way to cleanly
// shutdown the instance which is only available to calls from localhost.
func New() *Verto {
	v := Verto{
		Logger:     NewLogger(),
		verbose:    false,
		muxer:      mux.New(),
		Injections: NewInjections(),
	}

	// Reserve shutdown path
	v.muxer.AddFunc(
		"GET",
		"/shutdown",
		func(w http.ResponseWriter, r *http.Request) {
			ip := GetIP(r)
			if ip == "127.0.0.1" || ip == "::1" {
				v.sl.Stop()
			} else {
				v.muxer.NotFound.ServeHTTP(w, r)
			}
		})

	v.ErrorHandler = ErrorFunc(DefaultErrorFunc)
	v.ResponseHandler = ResponseFunc(DefaultResponseFunc)
	return &v
}

// Add registers a specific method+path combination to
// a resource function and returns an Endpoint representing
// said resource
func (v *Verto) Add(
	method, path string,
	rf ResourceFunc) *Endpoint {

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		c := NewContext(w, r, v.Injections, v.Logger)
		response, err := rf(c)
		if err != nil {
			v.ErrorHandler.Handle(err, c)
		} else {
			v.ResponseHandler.Handle(response, c)
		}
	}

	return &Endpoint{v.muxer.AddFunc(method, path, handlerFunc), v}
}

// AddHandler registers a specific method+path combination to
// an http.Handler and returns an Endpoint representing said
// resource
func (v *Verto) AddHandler(
	method, path string,
	handler http.Handler) *Endpoint {

	return &Endpoint{v.muxer.Add(method, path, handler), v}
}

func (v *Verto) Group(method, path string) *Group {
	return &Group{v.muxer.Group(method, path), v}
}

// Get is a wrapper function around Add() that sets the method
// as GET
func (v *Verto) Get(path string, rf ResourceFunc) *Endpoint {
	return v.Add("GET", path, rf)
}

// GetHandler is a wrapper function around AddHandler() that sets
// the method as GET
func (v *Verto) GetHandler(path string, handler http.Handler) *Endpoint {
	return v.AddHandler("GET", path, handler)
}

// Put is a wrapper function around Add() that sets the method
// as PUT
func (v *Verto) Put(path string, rf ResourceFunc) *Endpoint {
	return v.Add("PUT", path, rf)
}

// PutHandler is a wrapper function around AddHandler() that sets the method
// as PUT
func (v *Verto) PutHandler(path string, handler http.Handler) *Endpoint {
	return v.AddHandler("PUT", path, handler)
}

// Post is a wrapper function around Add() that sets the method
// as POST
func (v *Verto) Post(path string, rf ResourceFunc) *Endpoint {
	return v.Add("POST", path, rf)
}

// PostHandler is a wrapper function around AddHandler() that sets the method
// as POST
func (v *Verto) PostHandler(path string, handler http.Handler) *Endpoint {
	return v.AddHandler("POST", path, handler)
}

// Delete is a wrapper function around Add() that sets the method
// as DELETE
func (v *Verto) Delete(path string, rf ResourceFunc) *Endpoint {
	return v.Add("DELETE", path, rf)
}

// DeleteHandler is a wrapper function around AddHandler() that sets the method
// as DELETE
func (v *Verto) DeleteHandler(path string, handler http.Handler) *Endpoint {
	return v.AddHandler("DELETE", path, handler)
}

// SetVerbose sets whether the Verto instance is verbose or not.
func (v *Verto) SetVerbose(verbose bool) {
	v.verbose = verbose
}

// SetStrict sets whether to do strict path matching or not. If false,
// Verto will attempt to redirect trailing slashes to non-trailing slash
// paths if they exist and vice versa. The default is true which means
// Verto treats trailing slash as a different path than non-trailing slash
func (v *Verto) SetStrict(strict bool) {
	v.muxer.Strict = strict
}

// Use registers a mux.PluginHandler as a global plugin.
// to run for all groups and paths registered to the Verto instance.
// Plugins are called in order of definition.
func (v *Verto) Use(handler mux.PluginHandler) *Verto {
	v.muxer.Use(handler)
	return v
}

// UseHandler wraps an http.Handler as a mux.PluginHandler and calls Verto.Use().
func (v *Verto) UseHandler(handler http.Handler) *Verto {
	v.muxer.UseHandler(handler)
	return v
}

// UseVerto wraps a Plugin as a mux.PluginHandler and calls Verto.Use().
func (v *Verto) UseVerto(plugin Plugin) *Verto {
	pluginFunc := func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		c := NewContext(w, r, v.Injections, v.Logger)
		plugin.Handle(c, next)
	}
	v.Use(mux.PluginFunc(pluginFunc))
	return v
}

// RunOn runs Verto on the specified address (e.g. ":8080").
// RunOn by defaults adds a shutdown endpoint for Verto
// at /shutdown which can only be called locally.
func (v *Verto) RunOn(addr string) {
	if v.verbose {
		v.Logger.Info("Server initializing...")
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	v.sl, _ = WrapListener(listener)

	server := http.Server{
		Handler: v.muxer,
	}
	server.Serve(v.sl)

	if v.verbose {
		v.Logger.Info("Server shutting down.")
	}
}

// Run runs Verto on address ":8080".
func (v *Verto) Run() {
	v.RunOn(":8080")
}

// -------------------------------
// ---------- Helpers ------------

// DefaultErrorFunc is the default error handling
// function for Verto. DefaultErrorFunc sends a 500 response
// and writes the error's error message to the response body.
func DefaultErrorFunc(err error, c *Context) {
	c.Response.WriteHeader(500)
	fmt.Fprint(c.Response, err.Error())
}

// DefaultResponseFunc is the default response handling
// function for Verto. DefaultResponseFunc sends a 200 response and
// attempts to write the response directly to the http response body.
func DefaultResponseFunc(response interface{}, c *Context) {
	c.Response.WriteHeader(200)
	fmt.Fprint(c.Response, response)
}

// GetIP retrieves the ip address of the requester. GetIp recognizes
// the "X-Forwarded-For" header.
func GetIP(r *http.Request) string {
	if ip := r.Header.Get("x-forwarded-for"); len(ip) > 0 {
		return ip
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
