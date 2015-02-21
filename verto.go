// Verto
package verto

// Verto is a simple REST framework. It is
// plug n' play and includes it's own path
// multiplexer, error handler, and response
// handler. It is recommended to bring your
// own error handling and response handling.
// Verto provides users the option to use
// middleware globally or per route. The
// Verto multiplexer is currently not
// replaceable but that may change in the
// future.

import (
	"fmt"
	"github.com/boxtown/verto/mux"
	"net"
	"net/http"
)

// -------------------------------------------
// -------- Interfaces/Definitions -----------

// Interface for logging
type Logger interface {
	// Log a message to the default logger. Returns true if successful.
	Log(msg string) bool

	// Log a message to the named file. Returns true if successful.
	LogTo(name, msg string) bool
}

// Interface for error handling
type ErrorHandler interface {
	Handle(err error, c *Context)
}

// Function wrapper that implements ErrorHandler
type ErrorFunc func(err error, c *Context)

func (erf ErrorFunc) Handle(err error, c *Context) {
	erf(err, c)
}

// Interface for response handling
type ResponseHandler interface {
	Handle(response interface{}, c *Context)
}

// Function wrapper that implements ResponseHandler
type ResponseFunc func(response interface{}, c *Context)

func (rf ResponseFunc) Handle(response interface{}, c *Context) {
	rf(response, c)
}

// -----------------------------------
// ----------- Mux Wrapper -----------

// Custom plugin definition for Verto that allows injections by
// context.
type VertoPlugin interface {
	Handle(c *Context, next http.HandlerFunc)
}

// VertoPluginFunc wraps functions as Verto Plugins
type VertoPluginFunc func(c *Context, next http.HandlerFunc)

func (vpf VertoPluginFunc) Handle(c *Context, next http.HandlerFunc) {
	vpf(c, next)
}

// A wrapper around mux.Node that allows the use of Verto plugins.
type MuxWrapper struct {
	mux.Node
}

// Wrapper around mux.Node.Use that returns a MuxWrapper instead
// of a mux.Node
func (mw *MuxWrapper) Use(handler mux.PluginHandler) *MuxWrapper {
	return &MuxWrapper{mw.Node.Use(handler)}
}

// Wrapper around mux.Node.UseHandler that returns a MuxWrapper instead
// of a mux.Node
func (mw *MuxWrapper) UseHandler(handler http.Handler) *MuxWrapper {
	return &MuxWrapper{mw.Node.UseHandler(handler)}
}

// Wraps a VertoPlugin as a mux.PluginHandler and injects v's injections
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
			Logger:     v.logger,
		}

		plugin.Handle(c, next)
	}

	return &MuxWrapper{mw.Node.Use(mux.PluginFunc(pluginFunc))}
}

// function for Verto resource handlers
type ResourceFunc func(c *Context) (interface{}, error)

// ----------------------------
// ---------- Verto -----------

// Manages the routing and handling of all requests.
type Verto struct {
	muxer           *mux.PathMuxer
	logger          Logger
	injections      map[string]interface{}
	errorHandler    ErrorHandler
	responseHandler ResponseHandler
	doLogging       bool
}

// Returns a pointer to an initialized ResourceManager.
// You only need one resource manager per web application.
func New() *Verto {
	v := Verto{
		muxer:      mux.New(),
		injections: make(map[string]interface{}),
		logger:     nil,
		doLogging:  false,
	}

	v.errorHandler = ErrorFunc(DefaultErrorHandlerFunc)
	v.responseHandler = ResponseFunc(DefaultResponseHandlerFunc)

	return &v
}

// Inject a new Injection into the global context
func (v *Verto) Inject(tag string, injection interface{}) *Verto {
	v.injections[tag] = injection
	return v
}

// Use a global plugin. Plugins are called in order of definition.
// This function is just a wrapper for the muxer's global plugin chain.
func (v *Verto) Use(handler mux.PluginHandler) *Verto {
	v.muxer.Use(handler)
	return v
}

// Wraps an http Handler as a PluginHandler and calls Verto.Use().
func (v *Verto) UseHandler(handler http.Handler) *Verto {
	v.muxer.UseHandler(handler)
	return v
}

// Wraps a VertoPlugin as a PluginHandler and calls Verto.Use().
func (v *Verto) UseVerto(plugin VertoPlugin) *Verto {
	pluginFunc := func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		c := &Context{
			Response:   w,
			Request:    r,
			Injections: v.injections,
			Logger:     v.logger,
		}

		plugin.Handle(c, next)
	}
	v.Use(mux.PluginFunc(pluginFunc))

	return v
}

// Register a specific method+path combination to
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
			Logger:     v.logger,
		}

		response, err := rf(c)
		if err != nil {
			if v.doLogging {
				v.logger.Log(err.Error())
			}
			v.errorHandler.Handle(err, c)
		} else {
			v.responseHandler.Handle(response, c)
		}
	}

	return &MuxWrapper{v.muxer.AddFunc(method, path, handlerFunc)}
}

// Register a specific method+path combination to
// an http.Handler.
func (v *Verto) RegisterHandler(
	method, path string,
	handler http.Handler) *MuxWrapper {

	return &MuxWrapper{v.muxer.Add(method, path, handler)}
}

// Register a logger to Verto.
// This will replace but not close the existing logger.
func (v *Verto) RegisterLogger(logger Logger) {
	v.logger = logger
}

// Register an ErrorHandler to Verto.
// If no handler is registered, DefaultErrorHandler is used.
func (v *Verto) RegisterErrorHandler(errorHandler ErrorHandler) {
	v.errorHandler = errorHandler
}

// Register a ResponseHandler to Verto.
// If no handler is registered, DefaultResponseHandler is used.
func (v *Verto) RegisterResponseHandler(responseHandler ResponseHandler) {
	v.responseHandler = responseHandler
}

// Sets whether to do strict path matching or not.
func (v *Verto) SetStrict(strict bool) {
	v.muxer.Strict = strict
}

func (v *Verto) RunOn(addr string) {
	if v.doLogging {
		v.logger.Log("Server initializing...")
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
			ip := GetIp(r)
			if ip == "127.0.0.1" || ip == "::1" {
				sl.Stop()
			} else {
				mux.NotFoundHandler{}.ServeHTTP(w, r)
			}
		})

	server := http.Server{
		Handler: v.muxer,
	}
	server.Serve(sl)

	if v.doLogging {
		v.logger.Log("Server shutting down.")
	}
}

func (v *Verto) Run() {
	v.RunOn(":8080")
}

// Sets whether Verto logs or not.
func (v *Verto) SetLogging(log bool) {
	v.doLogging = log
}

// -------------------------------
// ---------- Helpers ------------

// Default error handler
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

// Default response handler
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

func GetIp(r *http.Request) string {
	if ip := r.Header.Get("x-forwarded-for"); len(ip) > 0 {
		return ip
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
