// Package Verto is a simple REST framework. It is
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

// -----------------------------------
// ----------- Mux Wrapper -----------

// VertoPlugin is a custom plugin definition for Verto that allows injections by
// context.
type VertoPlugin interface {
	Handle(c *Context, next http.HandlerFunc)
}

// VertoPluginFunc wraps functions as Verto Plugins
type VertoPluginFunc func(c *Context, next http.HandlerFunc)

// Handle calls functions wrapped by VertoPluginFunc.
func (vpf VertoPluginFunc) Handle(c *Context, next http.HandlerFunc) {
	vpf(c, next)
}

// MuxWrapper is a wrapper around mux.Node that allows the use of Verto plugins.
type MuxWrapper struct {
	mux.Node
}

// Use is a wrapper around mux.Node.Use that returns a MuxWrapper instead
// of a mux.Node
func (mw *MuxWrapper) Use(handler mux.PluginHandler) *MuxWrapper {
	return &MuxWrapper{mw.Node.Use(handler)}
}

// UseHandler is a wrapper around mux.Node.UseHandler that returns a MuxWrapper instead
// of a mux.Node
func (mw *MuxWrapper) UseHandler(handler http.Handler) *MuxWrapper {
	return &MuxWrapper{mw.Node.UseHandler(handler)}
}

// UseVerto wraps a VertoPlugin as a mux.PluginHandler and injects v's injections
// into the context. Returns the MuxWrapper for chaining.
func (mw *MuxWrapper) UseVerto(v *Verto, plugin VertoPlugin) *MuxWrapper {
	if v == nil {
		panic("MuxWrapper.UseVerto: Required Verto is nil")
	}

	pluginFunc := func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: v.injections,
			Logger:     v.Logger,
		}

		plugin.Handle(c, next)
	}

	return &MuxWrapper{mw.Node.Use(mux.PluginFunc(pluginFunc))}
}

// ResourceFunc is the Verto-specifc function for Verto resource handling.
type ResourceFunc func(c *Context) (interface{}, error)

// ----------------------------
// ---------- Verto -----------

// Verto is the framework that runs your app.
type Verto struct {
	Logger    Logger
	doLogging bool

	muxer      *mux.PathMuxer
	injections *Injections

	errorHandler    ErrorHandler
	responseHandler ResponseHandler
}

// New returns a pointer to a newly initialized Verto instance.
func New() *Verto {
	v := Verto{
		Logger:    NewLogger(),
		doLogging: false,

		muxer:      mux.New(),
		injections: NewInjections(),
	}

	v.errorHandler = ErrorFunc(DefaultErrorHandlerFunc)
	v.responseHandler = ResponseFunc(DefaultResponseHandlerFunc)

	return &v
}

// Inject injects a new Injection into the global context
func (v *Verto) Inject(tag string, injection interface{}) *Verto {
	v.injections.Set(tag, injection)
	return v
}

// Uninject clears an injection from the global context
func (v *Verto) Uninject(tag string) *Verto {
	v.injections.Delete(tag)
	return v
}

// ClearInjections clears the global context of all injections
func (v *Verto) ClearInjections() *Verto {
	v.injections.Clear()
	return v
}

// Register registers a specific method+path combination to
// a resource function. Any function registered using
// Register() can be assured the Context will not be null
func (v *Verto) Register(
	method, path string,
	rf ResourceFunc) *MuxWrapper {

	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: v.injections,
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

	return &MuxWrapper{v.muxer.AddFunc(method, path, handlerFunc)}
}

// RegisterHandler registers a specific method+path combination to
// an http.Handler.
func (v *Verto) RegisterHandler(
	method, path string,
	handler http.Handler) *MuxWrapper {

	return &MuxWrapper{v.muxer.Add(method, path, handler)}
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
func (v *Verto) UseVerto(plugin VertoPlugin) *Verto {
	pluginFunc := func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: v.injections,
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
	sl, _ := WrapListener(listener)

	v.muxer.AddFunc(
		"GET",
		"/shutdown",
		func(w http.ResponseWriter, r *http.Request) {
			ip := GetIP(r)
			if ip == "127.0.0.1" || ip == "::1" {
				sl.Stop()
			} else {
				v.muxer.NotFound.ServeHTTP(w, r)
			}
		})

	server := http.Server{
		Handler: v.muxer,
	}
	server.Serve(sl)

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
