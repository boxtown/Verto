package cors

import (
	"github.com/boxtown/verto"
	"github.com/boxtown/verto/plugins"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Options is a struct containing Cors plugin
// configuration options. MaxAge, if included, must
// be at least 1 second long. AllowedOrigins, AllowedHeaders,
// and AllowedMethods all support the wildcard designation '*'.
// If a wildcard is included, it should be the only string in
// the slice as it renders all other strings meaningless.
//
// Note: It is good security practice to explicitly define
// allowed origins, methods and headers instead of relying
// on a wildcard.
type Options struct {
	// AllowedOrigins designates a series of origins
	// as allowable for the 'Origin' header of incoming
	// requests. AllowedOrigins recognizes the wildcard
	// designation '*'. If AllowedOriginsFn is included,
	// it takes precedence over AllowedOrigins.
	AllowedOrigins []string

	// AllowedOriginsFn is a function that takes in an
	// origin and returns if it is allowable. If this
	// function is non-nil, it takes precedence over AllowedOrigins
	AllowedOriginsFn func(string) bool

	// ExposedHeaders designates a series of headers for the server
	// to expose in the 'Access-Control-Expose-Headers' header
	ExposedHeaders []string

	// AllowedHeaders designates a series of headers as allowable
	// for the 'Access-Control-Requested-Headers' header of incoming
	// requests. AllowedHeaders recognizes the wildcard designation '*'.
	// If AllowedHeadersFn is included, it takes precedence over AllowedHeaders
	AllowedHeaders []string

	// AllowedHeadersFn is a function that takes in a series of headers and
	// returns if they are allowable. If this function is non-nil, it takes
	// precedence over AllowedHeaders
	AllowedHeadersFn func([]string) bool

	// AllowedMethods designates a series of methods as allowable, either
	// per the request method for direct requests or per the 'Access-Control-Request-Method'
	// header on preflight requests. AllowedMethods recognizes the wildcard designation '*'.
	AllowedMethods []string

	// MaxAge is an optional field that designates the duration in seconds of
	// the 'Access-Control-Max-Age' header for preflight requests. If included,
	// MaxAge must be at least 1 second in duration
	MaxAge time.Duration

	// AllowCredentials is an optional field that sets the 'Access-Control-Allow-Credentials' header
	AllowCredentials bool
}

// Cors is the verto plugin that handles CORS requests based on a given
// configuration.
//
// Example usage:
//	cors := NewCors().Configure(&CorsOptions{
//		AllowedOrigins: []string{"*"},
//		AllowedHeadersFn:  func(h []string) bool {
//			// This is functionally equivalent
//			// to AllowHeaders: []string{"*"}
//			return true
//		},
//		AllowedMethods: []string{"GET", "POST"}
//	})
type Cors struct {
	plugins.Core

	allowedOrigins   map[string]bool
	allowedOriginsFn func(string) bool
	exposedHeaders   []string
	allowedHeaders   map[string]bool
	allowedHeadersFn func([]string) bool
	allowedMethods   map[string]bool
	maxAge           int64
	allowCredentials bool
	configured       bool
}

// NewCors returns a new Cors plugin instance that is unconfigured.
// It is best practice to call either the Configure or Default functions
// immediately on the newly instantiated plugin instance
func New() *Cors {
	return &Cors{Core: plugins.Core{Id: "plugins.Cors"}}
}

// Configure configures the Cors plugin according to the passed
// in options. Each consecutive call to Configure will first create
// a fresh instance of a cors plugin before configuring the plugin.
// As such, it is generally recommended to only call the Configure
// function once immediately after instantiating a new Cors plugin
// and to not mix the call with a call to Default.
//
// Example:
//	cors := NewCors().Configure(&CorsOptions{
// 		...
//	})
func (plugin *Cors) Configure(opts *Options) *Cors {
	// Each consecutive call to configure creates a fresh
	// plugin state
	p := plugin
	if plugin.configured {
		p = New()
	}

	// Set allowable origin handling logic
	if opts.AllowedOriginsFn != nil {
		p.allowedOriginsFn = opts.AllowedOriginsFn
	} else {
		for _, o := range opts.AllowedOrigins {
			p.allowedOrigins[clean(o)] = true
		}
	}

	// Set allowable header handling logic
	if opts.AllowedHeadersFn != nil {
		p.allowedHeadersFn = opts.AllowedHeadersFn
	} else {
		for _, h := range opts.AllowedHeaders {
			p.allowedHeaders[clean(h)] = true
		}
		// Origins header always allowed
		p.allowedHeaders["origins"] = true
	}

	// Set allowable methods logic
	for _, m := range opts.AllowedMethods {
		p.allowedMethods[clean(m)] = true
	}
	// OPTIONS preflight method is always allowed
	p.allowedMethods["options"] = true

	// If the Max-Age duration is valid (e.g. > 1 second),
	// set Max-Age
	if int64(opts.MaxAge/time.Second) > 1 {
		p.maxAge = int64(opts.MaxAge / time.Second)
	}

	// Set pass-through values
	p.exposedHeaders = opts.ExposedHeaders
	p.allowCredentials = opts.AllowCredentials
	p.configured = true
	return p
}

// Default configures a Cors instance to use sensible default options.
// Each consective call to Default will instantiate a fresh Cors plugin
// instance. As such, it is generally recommended to only call Default once
// after instantiating a new Cors plugin instance and to not mix the call
// with a call to Configure.
//
// Example:
//	cors := NewCors().Default()
func (plugin *Cors) Default() *Cors {
	return plugin.Configure(
		&Options{
			AllowedOrigins: []string{"*"},
			AllowedHeaders: []string{"Origin", "Accept", "Content-Type"},
			AllowedMethods: []string{"GET", "POST"},
			MaxAge:         time.Second * 60,
		})
}

// Handle is called per web request to handle the validation and writing
// of relevant CORS headers from incoming requests.
func (plugin *Cors) Handle(c *verto.Context, next http.HandlerFunc) {
	plugin.Core.Handle(
		func(c *verto.Context, next http.HandlerFunc) {
			r := c.Request
			w := c.Response

			pf := r.Method == "OPTIONS"
			plugin.writeHeaders(w, r, pf)
			if !pf {
				next(w, r)
			}
		}, c, next)
}

// checks request headers and method and, if all pass
// writes relevant response headers
func (plugin *Cors) writeHeaders(w http.ResponseWriter, r *http.Request, preflight bool) {
	w.Header().Add("Vary", "Origin")
	if preflight {
		w.Header().Add("Vary", "Access-Control-Request-Method")
		w.Header().Add("Vary", "Access-Control-Request-Headers")
	}

	// Check origin.
	origin := r.Header.Get("Origin")
	if !plugin.isOriginAllowed(origin) {
		return
	}

	// Check method
	method := r.Method
	if preflight {
		method = r.Header.Get("Access-Control-Request-Method")
	}
	if !plugin.isMethodAllowed(method) {
		return
	}

	// Check requested headers if preflight
	headers := r.Header.Get("Access-Control-Request-Headers")
	if preflight && !plugin.areHeadersAllowed(strings.Split(headers, ",")) {
		return
	}

	// Write relevant headers
	w.Header().Set("Access-Control-Allow-Origin", origin)
	if plugin.allowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if len(plugin.exposedHeaders) > 0 {
		w.Header().Set("Access-Control-Exposed-Headers", strings.Join(plugin.exposedHeaders, ","))
	}
	if preflight {
		w.Header().Set("Access-Control-Allow-Methods", method)
		w.Header().Set("Access-Control-Allow-Headers", headers)
		if plugin.maxAge > 0 {
			w.Header().Set("Access-Control-Max-Age", strconv.FormatInt(plugin.maxAge, 10))
		}
	}
}

func (plugin *Cors) isOriginAllowed(origin string) bool {
	if plugin.allowedOriginsFn != nil {
		return plugin.allowedOriginsFn(origin)
	}

	origin = clean(origin)
	return plugin.allowedOrigins[origin] || plugin.allowedOrigins[wc]
}

func (plugin *Cors) isMethodAllowed(method string) bool {
	method = clean(method)
	return plugin.allowedMethods[method] || plugin.allowedMethods[wc]
}

func (plugin *Cors) areHeadersAllowed(headers []string) bool {
	if plugin.allowedHeadersFn != nil {
		return plugin.allowedHeadersFn(headers)
	}
	if plugin.allowedHeaders[wc] {
		return true
	}

	for _, h := range headers {
		h := clean(h)
		if !plugin.allowedHeaders[h] {
			return false
		}
	}
	return true
}

func clean(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

const wc string = "*"
