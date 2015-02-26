package verto

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
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

	params url.Values

	// This field is populated by Verto based on user
	// set injections.
	Injections *Injections

	// If Verto has a registered Logger, it can be
	// accessed here.
	Logger Logger
}

// Get retrieves the request parameter associated with
// key or an error if Context was not properly initialized.
func (c *Context) Get(key string) (string, error) {
	if c.Request == nil {
		return "", ErrContextNotInitialized
	}

	if c.params == nil {
		c.params = c.Request.URL.Query()
	}

	return c.params.Get(key), nil
}

// GetMulti returns the a slice containing all relevant parameters
// tied to key or an error if Context was not properly initialized.
func (c *Context) GetMulti(key string) ([]string, error) {
	if c.Request == nil {
		return nil, ErrContextNotInitialized
	}

	if c.params == nil {
		c.params = c.Request.URL.Query()
	}

	return c.params[key], nil
}

// GetBool retrieves the value associated with key as a bool
// or returns an error if the conversion failed or Context
// was not properly initialized.
func (c *Context) GetBool(key string) (bool, error) {
	v, err := c.Get(key)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(v)
}

// GetFloat64 retrieves the value associated with key as
// a float64 or returns an error if the conversion failed or
// Context was not properly initialized.
func (c *Context) GetFloat64(key string) (float64, error) {
	v, err := c.Get(key)
	if err != nil {
		return float64(0), err
	}
	return strconv.ParseFloat(v, 64)
}

// GetInt64 retrieves the value associated with key as
// an int64 or returns an error if the conversion failed or
// Context was not properly initialized.
func (c *Context) GetInt64(key string) (int64, error) {
	v, err := c.Get(key)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

// Set associates a request parameter value with key.
// Set returns an error if Context was not properly
// initialized.
func (c *Context) Set(key, value string) error {
	if c.Request == nil {
		return ErrContextNotInitialized
	}

	if c.params == nil {
		c.params = c.Request.URL.Query()
	}

	c.params.Set(key, value)
	c.Request.URL.RawQuery = c.params.Encode()
	c.params = c.Request.URL.Query()

	return nil
}

// SetMulti associates multiple parameter values with key.
// SetMulti returns an error if Context was not properly
// initialized.
func (c *Context) SetMulti(key string, values []string) error {
	if c.Request == nil {
		return ErrContextNotInitialized
	}

	if c.params == nil {
		c.params = c.Request.URL.Query()
	}

	for _, v := range values {
		c.params.Add(key, v)
	}
	c.Request.URL.RawQuery = c.params.Encode()
	c.params = c.Request.URL.Query()

	return nil
}

// SetBool attempts to associate value with key and returns
// an error if the conversion failed or Context was not properly
// initialized.
func (c *Context) SetBool(key string, value bool) error {
	v := strconv.FormatBool(value)
	return c.Set(key, v)
}

// SetFloat64 attempts to associate value with key and returns
// an error if the conversion failed or Context was not properly
// initialized.
func (c *Context) SetFloat64(key string, value float64, fmt byte, prec int) error {
	v := strconv.FormatFloat(value, fmt, prec, 64)
	return c.Set(key, v)
}

// SetInt64 attempts to associate value with key and returns
// an error if the conversion failed or Context was not properly
// initialized.
func (c *Context) SetInt64(key string, value int64) error {
	v := strconv.FormatInt(value, 10)
	return c.Set(key, v)
}
