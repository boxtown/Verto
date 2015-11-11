package verto

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

// ErrContextNotInitialized is thrown by Context Get/Set utility functions
// if the Context was not properly initialized. Contexts passed to request
// handlers and plugins are guaranteed to be properly initialized.
var ErrContextNotInitialized = errors.New("context not initialized")

// Context contains useful state information for request handling.
// Inside Context is the original http.ResponseWriter and *http.Request
// as well as access to a Logger and Injections.
type Context struct {
	// The original ResponseWriter
	Response http.ResponseWriter

	// The original *http.Request
	Request *http.Request

	// This field is populated by Verto based on user
	// set injections.
	Injections *Injections

	// If Verto has a registered Logger, it can be
	// accessed here.
	Logger Logger

	params   url.Values
	parseErr error
	mut      *sync.Mutex
}

// NewContext initializes a new Context with the passed in response, request,
// injections, and logger
func NewContext(w http.ResponseWriter, r *http.Request, i *Injections, l Logger) *Context {
	return &Context{
		Response:   w,
		Request:    r,
		Injections: i,
		Logger:     l,
		mut:        &sync.Mutex{},
	}
}

// Get retrieves the request parameter associated with
// key. If there was an error retrieving the parameter,
// the error is stored and retrievable by the ParseError
// call.
func (c *Context) Get(key string) string {
	c.mut.Lock()
	defer c.mut.Unlock()

	if c.Request == nil {
		c.parseErr = ErrContextNotInitialized
		return ""
	}
	if c.params == nil {
		if err := c.Request.ParseForm(); err != nil {
			c.parseErr = err
		}
		c.params = c.Request.Form
	}
	return c.params.Get(key)
}

// GetMulti returns the a slice containing all relevant parameters
// tied to key. If there was an error retrieving the parameters,
// the error is stored and retrievable by the ParseError call
func (c *Context) GetMulti(key string) []string {
	c.mut.Lock()
	defer c.mut.Unlock()

	if c.Request == nil {
		c.parseErr = ErrContextNotInitialized
		return nil
	}
	if c.params == nil {
		if err := c.Request.ParseForm(); err != nil {
			c.parseErr = err
		}
		c.params = c.Request.Form
	}
	return c.params[key]
}

// GetBool retrieves the value associated with key as a bool
// or returns an error if the conversion failed
func (c *Context) GetBool(key string) (bool, error) {
	v := c.Get(key)
	return strconv.ParseBool(v)
}

// GetFloat64 retrieves the value associated with key as
// a float64 or returns an error if the conversion failed
func (c *Context) GetFloat64(key string) (float64, error) {
	v := c.Get(key)
	return strconv.ParseFloat(v, 64)
}

// GetInt64 retrieves the value associated with key as
// an int64 or returns an error if the conversion failed
func (c *Context) GetInt64(key string) (int64, error) {
	v := c.Get(key)
	return strconv.ParseInt(v, 10, 64)
}

// Set associates a request parameter value with key.
func (c *Context) Set(key, value string) {
	c.mut.Lock()
	defer c.mut.Unlock()

	if c.Request == nil {
		c.parseErr = ErrContextNotInitialized
		return
	}
	if c.params == nil {
		if err := c.Request.ParseForm(); err != nil {
			c.parseErr = err
		}
		c.params = c.Request.Form
	}
	c.params.Set(key, value)
}

// SetMulti associates multiple parameter values with key.
func (c *Context) SetMulti(key string, values []string) {
	c.mut.Lock()
	defer c.mut.Unlock()

	if c.Request == nil {
		c.parseErr = ErrContextNotInitialized
		return
	}
	if c.params == nil {
		if err := c.Request.ParseForm(); err != nil {
			c.parseErr = err
		}
		c.params = c.Request.Form
	}

	for _, v := range values {
		c.params.Add(key, v)
	}
}

// SetBool converts a boolean value to a string and
// associates it with a key in the context
func (c *Context) SetBool(key string, value bool) {
	v := strconv.FormatBool(value)
	c.Set(key, v)
}

// SetFloat64 converts a float64 to a string and associates it
// with a key in the context
func (c *Context) SetFloat64(key string, value float64, fmt byte, prec int) {
	v := strconv.FormatFloat(value, fmt, prec, 64)
	c.Set(key, v)
}

// SetInt64 converts an int64 to a (base 10 representation) string and associates
// it with a key in the context
func (c *Context) SetInt64(key string, value int64) {
	v := strconv.FormatInt(value, 10)
	c.Set(key, v)
}

// ParseError returns the error encountered while parsing
// the HTTP request for parameter values or nil if no
// error was encountered
func (c *Context) ParseError() error {
	return c.parseErr
}
