// Package verto is a simple REST framework. It is
// plug n' play and includes it's own path
// multiplexer, error handler, and response
// handler. It is recommended to bring your
// own error handling and response handling.
// Verto provides users the option to use
// middleware globally or per route. The
// Verto multiplexer is currently not
// replaceable but that may change in the
// future.
package verto

import (
	"fmt"
	"github.com/boxtown/verto/mux"
	"net"
	"net/http"
)

// -------------------------------------------
// -------- Interfaces/Definitions -----------

// ErrorHandler is the Verto-specific interface for error handlers
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

// ResponseHandler is the Verto-specific interface for response handlers
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

// GroupWrapper is a wrapper around mux.Group that allows the use of Verto plugins
type GroupWrapper struct {
	g mux.Group
	v *Verto
}

// Add is a wrapper around mux.Group.Add that returns a NodeWrapper
// instead of a mux.Node
func (gw *GroupWrapper) Add(
	method, path string,
	rf ResourceFunc) *NodeWrapper {

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: gw.v.Injections,
			Logger:     gw.v.Logger,
		}

		response, err := rf(c)
		if err != nil {
			if gw.v.doLogging {
				gw.v.Logger.Error(err.Error())
			}
			gw.v.errorHandler.Handle(err, c)
		} else {
			gw.v.responseHandler.Handle(response, c)
		}
	}

	return &NodeWrapper{gw.g.AddFunc(method, path, handlerFunc), gw.v}
}

// AddHandler is a wrapper around mux.Group.Add that returns a NodeWrapper
// instead of a mux.Node
func (gw *GroupWrapper) AddHandler(
	method, path string,
	handler http.Handler) *NodeWrapper {

	return &NodeWrapper{gw.g.Add(method, path, handler), gw.v}
}

// Group is a wrapper around mux.Group.Group that returns a GroupWrapper
// instead of a mux.Group
func (gw *GroupWrapper) Group(path string) *GroupWrapper {
	return &GroupWrapper{gw.g.Group(path), gw.v}
}

// Use is a wrapper around mux.Group.Use that returns a GroupWrapper
// instead of a mux.Group
func (gw *GroupWrapper) Use(handler mux.PluginHandler) *GroupWrapper {
	return &GroupWrapper{gw.g.Use(handler), gw.v}
}

// UseHandler is a wrapper around mux.Group.UseHandler that returns a GroupWrapper
// instead of a mux.Group
func (gw *GroupWrapper) UseHandler(handler http.Handler) *GroupWrapper {
	return &GroupWrapper{gw.g.UseHandler(handler), gw.v}
}

// UseVerto wraps a VertoPlugin as a mux.PluginHandler and injects v's injections
// into the context. Returns the GroupWrapper for chaining.
func (gw *GroupWrapper) UseVerto(plugin Plugin) *GroupWrapper {
	pluginFunc := func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: gw.v.Injections,
			Logger:     gw.v.Logger,
		}

		plugin.Handle(c, next)
	}

	return &GroupWrapper{gw.g.Use(mux.PluginFunc(pluginFunc)), gw.v}
}

// NodeWrapper is a wrapper around mux.Node that allows the use of Verto plugins.
type NodeWrapper struct {
	n mux.Node
	v *Verto
}

// Use is a wrapper around mux.Node.Use that returns a NodeWrapper instead
// of a mux.Node
func (nw *NodeWrapper) Use(handler mux.PluginHandler) *NodeWrapper {
	return &NodeWrapper{nw.n.Use(handler), nw.v}
}

// UseHandler is a wrapper around mux.Node.UseHandler that returns a NodeWrapper instead
// of a mux.Node
func (nw *NodeWrapper) UseHandler(handler http.Handler) *NodeWrapper {
	return &NodeWrapper{nw.n.UseHandler(handler), nw.v}
}

// UseVerto wraps a VertoPlugin as a mux.PluginHandler and injects v's injections
// into the context. Returns the NodeWrapper for chaining.
func (nw *NodeWrapper) UseVerto(plugin Plugin) *NodeWrapper {
	pluginFunc := func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: nw.v.Injections,
			Logger:     nw.v.Logger,
		}

		plugin.Handle(c, next)
	}

	return &NodeWrapper{nw.n.Use(mux.PluginFunc(pluginFunc)), nw.v}
}

// ResourceFunc is the Verto-specifc function for Verto resource handling.
type ResourceFunc func(c *Context) (interface{}, error)

// ----------------------------
// ---------- Verto -----------

// Verto is the framework that runs your app.
type Verto struct {
	Logger    Logger
	doLogging bool

	sl    *StoppableListener
	muxer *mux.PathMuxer

	Injections *Injections

	errorHandler    ErrorHandler
	responseHandler ResponseHandler
}

// New returns a pointer to a newly initialized Verto instance.
func New() *Verto {
	v := Verto{
		Logger:    NewLogger(),
		doLogging: false,

		muxer: mux.New(),

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

	v.errorHandler = ErrorFunc(DefaultErrorHandlerFunc)
	v.responseHandler = ResponseFunc(DefaultResponseHandlerFunc)

	return &v
}

// Add registers a specific method+path combination to
// a resource function. Any function registered using
// Add() can be assured the Context will not be null
func (v *Verto) Add(
	method, path string,
	rf ResourceFunc) *NodeWrapper {

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: v.Injections,
			Logger:     v.Logger,
		}

		response, err := rf(c)
		if err != nil {
			if v.doLogging {
				v.Logger.Error(err.Error())
			}
			v.errorHandler.Handle(err, c)
		} else {
			v.responseHandler.Handle(response, c)
		}
	}

	return &NodeWrapper{v.muxer.AddFunc(method, path, handlerFunc), v}
}

// AddHandler registers a specific method+path combination to
// an http.Handler.
func (v *Verto) AddHandler(
	method, path string,
	handler http.Handler) *NodeWrapper {

	return &NodeWrapper{v.muxer.Add(method, path, handler), v}
}

// Group creates a group at the specified path. The group's plugins get run
// for all subgroups and endpoints under the group. Creating a group
// will automatically subsume any sub-groups and endpoints that already exist
// and share a path prefix with the group. Subsumed paths with wildcards in the
// shared prefix will use the new group's wildcard key instead. Attempting to create
// an already existing group just returns the existing group.
func (v *Verto) Group(path string) *GroupWrapper {
	return &GroupWrapper{v.muxer.Group(path), v}
}

// RegisterLogger register a Logger to Verto.
func (v *Verto) RegisterLogger(Logger Logger) {
	v.Logger = Logger
}

// RegisterErrorHandler registers an ErrorHandler to Verto.
// If no handler is registered, DefaultErrorHandler is used.
func (v *Verto) RegisterErrorHandler(errorHandler ErrorHandler) {
	v.errorHandler = errorHandler
}

// RegisterResponseHandler registers a ResponseHandler to Verto.
// If no handler is registered, DefaultResponseHandler is used.
func (v *Verto) RegisterResponseHandler(responseHandler ResponseHandler) {
	v.responseHandler = responseHandler
}

// SetLogging sets whether Verto logs or not.
func (v *Verto) SetLogging(log bool) {
	v.doLogging = log
}

// SetStrict sets whether to do strict path matching or not.
func (v *Verto) SetStrict(strict bool) {
	v.muxer.Strict = strict
}

// Use registers a global plugin. Plugins are called in order of definition.
// This function is just a wrapper for the muxer's global plugin chain.
func (v *Verto) Use(handler mux.PluginHandler) *Verto {
	v.muxer.Use(handler)
	return v
}

// UseHandler wraps an http.Handler as a PluginHandler and calls Verto.Use().
func (v *Verto) UseHandler(handler http.Handler) *Verto {
	v.muxer.UseHandler(handler)
	return v
}

// UseVerto wraps a VertoPlugin as a PluginHandler and calls Verto.Use().
func (v *Verto) UseVerto(plugin Plugin) *Verto {
	pluginFunc := func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: v.Injections,
			Logger:     v.Logger,
		}

		plugin.Handle(c, next)
	}
	v.Use(mux.PluginFunc(pluginFunc))

	return v
}

// RunOn runs Verto on the specified address (e.g. ":8080").
// RunOn by defaults adds a shutdown endpoint for Verto
// at /shutdown which can only be called locally.
func (v *Verto) RunOn(addr string) {
	if v.doLogging {
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

	if v.doLogging {
		v.Logger.Info("Server shutting down.")
	}
}

// Run runs Verto on address ":8080".
func (v *Verto) Run() {
	v.RunOn(":8080")
}

// -------------------------------
// ---------- Helpers ------------

// DefaultErrorHandlerFunc is the default error handling
// function for Verto. DefaultErrorHandlerFunc sends a 500 response
// and the error message as the response body.
func DefaultErrorHandlerFunc(err error, c *Context) {
	if c == nil {
		return
	}
	if c.Response == nil {
		return
	}

	c.Response.WriteHeader(500)
	fmt.Fprint(c.Response, err.Error())
}

// DefaultResponseHandlerFunc is the default response handling
// function for Verto. DefaultResponseFunc sends a 200 response with
// response as the response body.
func DefaultResponseHandlerFunc(response interface{}, c *Context) {
	if c == nil {
		return
	}
	if c.Response == nil {
		return
	}

	c.Response.WriteHeader(200)
	fmt.Fprint(c.Response, response)
}

// GetIP retrieves the ip address of the requester. GetIp recognizes
// the "x-forwarded-for" header.
func GetIP(r *http.Request) string {
	if ip := r.Header.Get("x-forwarded-for"); len(ip) > 0 {
		return ip
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
