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

// Endpoint is a wrapper around mux.Endpoint that allows the use of Verto plugins.
type Endpoint struct {
	mux.Endpoint
	v *Verto
}

// Use is a wrapper around mux.Endpoint.Use that returns an Endpoint instead
// of a mux.Endpoint
func (ep *Endpoint) Use(handler mux.PluginHandler) *Endpoint {
	return &Endpoint{ep.Endpoint.Use(handler), ep.v}
}

// UseHandler is a wrapper around mux.Endpoint.UseHandler that returns an Endpoint instead
// of a mux.Endpoint
func (ep *Endpoint) UseHandler(handler http.Handler) *Endpoint {
	return &Endpoint{ep.Endpoint.UseHandler(handler), ep.v}
}

// UseVerto wraps a VertoPlugin as a mux.PluginHandler and injects v's injections
// into the context. Returns the Endpoint for chaining.
func (ep *Endpoint) UseVerto(plugin Plugin) *Endpoint {
	pluginFunc := func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: ep.v.Injections,
			Logger:     ep.v.Logger,
		}

		plugin.Handle(c, next)
	}
	return &Endpoint{ep.Endpoint.Use(mux.PluginFunc(pluginFunc)), ep.v}
}

// Group is a wrapper around mux.Group that allows the use of Verto plugins.
type Group struct {
	g mux.Group
	v *Verto
}

// Add is a wrapper around mux.Group.Add wraps rf as an http.Handler and
// returns an Endpoint instead of a mux.Endpoint
func (g *Group) Add(path string, rf ResourceFunc) *Endpoint {
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: g.v.Injections,
			Logger:     g.v.Logger,
		}

		response, err := rf(c)
		if err != nil {
			if g.v.doLogging {
				g.v.Logger.Error(err.Error())
			}
			g.v.ErrorHandler.Handle(err, c)
		} else {
			g.v.ResponseHandler.Handle(response, c)
		}
	}

	return &Endpoint{g.g.AddFunc(path, handlerFunc), g.v}
}

// AddHandler is a wrapper around mux.Group.Add that returns an
// Endpoint instead of a mux.Endpoint
func (g *Group) AddHandler(path string, handler http.Handler) *Endpoint {
	return &Endpoint{g.g.Add(path, handler), g.v}
}

// Group is a wrapper around mux.Group.Group that returns
// a Group instead of a mux.Group
func (g *Group) Group(path string) *Group {
	return &Group{g.g.Group(path), g.v}
}

// Use is a wrapper around mux.Group.Use that returns
// a Group instead of a mux.Group
func (g *Group) Use(handler mux.PluginHandler) *Group {
	return &Group{g.g.Use(handler), g.v}
}

// UseHandler is a wrapper around mux.Group.UseHandler that
// returns a Group instead of a mux.Group
func (g *Group) UseHandler(handler http.Handler) *Group {
	return &Group{g.g.UseHandler(handler), g.v}
}

// UseVerto wraps plugin as a mux.PluginFunc and calls
// mux.Group.Use. UseVerto returns the Group for chaining
// Use calls.
func (g *Group) UseVerto(plugin Plugin) *Group {
	pluginFunc := func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: g.v.Injections,
			Logger:     g.v.Logger,
		}

		plugin.Handle(c, next)
	}
	return &Group{g.g.Use(mux.PluginFunc(pluginFunc)), g.v}
}

// ResourceFunc is the Verto-specific function for endpoint resource handling.
type ResourceFunc func(c *Context) (interface{}, error)

// ----------------------------
// ---------- Verto -----------

// Verto is the framework that runs your application.
type Verto struct {
	Injections      *Injections
	Logger          Logger
	ErrorHandler    ErrorHandler
	ResponseHandler ResponseHandler

	doLogging bool
	sl        *StoppableListener
	muxer     *mux.PathMuxer
}

// VertoHTTPHandler is a wrapper around Verto such that it can run
// as an http.handler
type VertoHTTPHandler struct {
	*Verto
}

// ServeHTTP serves request directly to Verto's muxer.
func (vhh *VertoHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vhh.muxer.ServeHTTP(w, r)
}

// New returns a pointer to a newly initialized Verto instance.
// The path /shutdown is automatically reserved as a way to cleanly
// shutdown the instance but is only available to calls from localhost.
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

	v.ErrorHandler = ErrorFunc(DefaultErrorFunc)
	v.ResponseHandler = ResponseFunc(DefaultResponseFunc)

	return &v
}

// Add registers a specific method+path combination to
// a resource function. Any function registered using
// Add() can be assured the Context will not be null
func (v *Verto) Add(
	method, path string,
	rf ResourceFunc) *Endpoint {

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
			v.ErrorHandler.Handle(err, c)
		} else {
			v.ResponseHandler.Handle(response, c)
		}
	}

	return &Endpoint{v.muxer.AddFunc(method, path, handlerFunc), v}
}

// AddHandler registers a specific method+path combination to
// an http.Handler.
func (v *Verto) AddHandler(
	method, path string,
	handler http.Handler) *Endpoint {

	return &Endpoint{v.muxer.Add(method, path, handler), v}
}

func (v *Verto) Group(method, path string) *Group {
	return &Group{v.muxer.Group(method, path), v}
}

// Get is a wrapper function around Add() that sets the method
// as commonly GET
func (v *Verto) Get(path string, rf ResourceFunc) *Endpoint {
	return v.Add("GET", path, rf)
}

// GetHandler is a wrapper function around AddHandler() that sets
// the method as GET
func (v *Verto) GetHandler(path string, handler http.Handler) *Endpoint {
	return v.AddHandler("GET", path, handler)
}

// Put is a wrapper function around Add() that sets the method
// as commonly PUT
func (v *Verto) Put(path string, rf ResourceFunc) *Endpoint {
	return v.Add("PUT", path, rf)
}

// PutHandler is a wrapper function around AddHandler() that sets the method
// as commonly PUT
func (v *Verto) PutHandler(path string, handler http.Handler) *Endpoint {
	return v.AddHandler("PUT", path, handler)
}

// Post is a wrapper function around Add() that sets the method
// as commonly POST
func (v *Verto) Post(path string, rf ResourceFunc) *Endpoint {
	return v.Add("POST", path, rf)
}

// PostHandler is a wrapper function around AddHandler() that sets the method
// as commonly POST
func (v *Verto) PostHandler(path string, handler http.Handler) *Endpoint {
	return v.AddHandler("POST", path, handler)
}

// Delete is a wrapper function around Add() that sets the method
// as commonly DELETE
func (v *Verto) Delete(path string, rf ResourceFunc) *Endpoint {
	return v.Add("DELETE", path, rf)
}

// DeleteHandler is a wrapper function around AddHandler() that sets the method
// as commonly DELETE
func (v *Verto) DeleteHandler(path string, handler http.Handler) *Endpoint {
	return v.AddHandler("DELETE", path, handler)
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
		v.Logger.Close()
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
// and the error message as the response body.
func DefaultErrorFunc(err error, c *Context) {
	c.Response.WriteHeader(500)
	fmt.Fprint(c.Response, err.Error())
}

// DefaultResponseFunc is the default response handling
// function for Verto. DefaultResponseFunc sends a 200 response with
// response as the response body.
func DefaultResponseFunc(response interface{}, c *Context) {
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
