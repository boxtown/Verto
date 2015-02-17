// context
package verto

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

var ContextNotInitializedErr = errors.New("Context not initialized")

type Context struct {
	Response http.ResponseWriter
	Request  *http.Request

	params url.Values

	Injections map[string]interface{}
	Logger     Logger
}

func (c *Context) Get(key string) (string, error) {
	if c.Request == nil {
		return "", ContextNotInitializedErr
	}

	if c.params == nil {
		c.params = c.Request.URL.Query()
	}

	return c.params.Get(key), nil
}

func (c *Context) GetMulti(key string) ([]string, error) {
	if c.Request == nil {
		return nil, ContextNotInitializedErr
	}

	if c.params == nil {
		c.params = c.Request.URL.Query()
	}

	return c.params[key], nil
}

func (c *Context) GetBool(key string) (bool, error) {
	v, err := c.Get(key)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(v)
}

func (c *Context) GetFloat64(key string) (float64, error) {
	v, err := c.Get(key)
	if err != nil {
		return float64(0), err
	}
	return strconv.ParseFloat(v, 64)
}

func (c *Context) GetInt64(key string) (int64, error) {
	v, err := c.Get(key)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

func (c *Context) Set(key, value string) error {
	if c.Request == nil {
		return ContextNotInitializedErr
	}

	if c.params == nil {
		c.params = c.Request.URL.Query()
	}

	c.params.Set(key, value)
	c.Request.URL.RawQuery = c.params.Encode()
	c.params = c.Request.URL.Query()

	return nil
}

func (c *Context) SetMulti(key string, values []string) error {
	if c.Request == nil {
		return ContextNotInitializedErr
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

func (c *Context) SetBool(key string, value bool) error {
	v := strconv.FormatBool(value)
	return c.Set(key, v)
}

func (c *Context) SetFloat64(key string, value float64, fmt byte, prec int) error {
	v := strconv.FormatFloat(value, fmt, prec, 64)
	return c.Set(key, v)
}

func (c *Context) SetInt64(key string, value int64) error {
	v := strconv.FormatInt(value, 10)
	return c.Set(key, v)
}
