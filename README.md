# Verto

[![Build Status](https://travis-ci.org/boxtown/verto.svg?branch=master)](https://travis-ci.org/boxtown/verto) [![GoDoc](https://godoc.org/github.com/boxtown/verto?status.svg)](https://godoc.org/github.com/boxtown/verto)

#### [Examples](https://boxtown.io/#/projects/verto/dev)
#### [Benchmarks](https://boxtown.io/#/projects/verto/benchmarks)
  
Verto is a simple REST framework that provides routing, error handling, response handling,  
logging, and middleware chaining.  
  
  - [Basic Usage](#basic-usage)
  - [Resource Function](#resource-function)
  - [Routing](#routing)
  - [Path Redirection](#path-redirection)
  - [Middleware Chaining](#middleware-chaining)
  - [Groups](#groups)
  - [Context](#context)
  - [Response Handler](#response-handler)
  - [Error Handler](#error-handler)
  - [Injections](#injections)
  - [Logging](#logging)
  
### Basic Usage  
  ```Go
    // Initialize Verto
    v := verto.New()
    
    // Register a route
    v.Add("GET", "/", func(c *verto.Context) (interface{}, error) {
      fmt.Fprintf(c.Response, "Hello, World!")
    })
    
    // Run Verto
    v.Run()
  ```
  
### Resource Function
  
Verto accepts a custom ResourceFunction:  
  
  ```Go
    type ResourceFunc(c *Context) (interface{}, error) func
  ```
  
The function takes in a Verto [Context](#context) struct. The Context contains  
a `map` of [injections](#injections) that we will discuss later. The resource function  
returns an `interface{}` and optionally an `error`. If the call to the resource was successful,  
a nil `error` should be returned. The default Verto instance will be able to handle any `error`s  
or `interface{}` return values but in a very basic way. It is recommended for the user to bring  
a custom [response](#response-handler) and [error](#error-handler) handler to Verto.  
  
Verto will also happily take `http.Handler` as a routing endpoint.  
  
### Routing  
  
Verto offers simple routing. Endpoints are registered using the `Add` and `AddHandker`  
methods. The parameters are the method to be matched, the path to be matched, and the endpoint  
handler. `Add` takes a `verto.ResourceFunc` as an endpoint and `AddHandler` takes a  
normal `http.Handler`. 
  
  ```Go
    // Basic routing example
    endpoint1 := verto.ResourceFunc(c *verto.Context) (interface{}, error) {
      fmt.Fprintf(c.Response, "Hello, World!")
    })
    endpoint2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      fmt.Fprintf(w, "Hello, World!")
    })
    
    v.Add("GET", "/path/to/1", endpoint1)
    v.Add("POST", "/path/to/1", endpoint1)
    
    v.AddHandler("PUT", "/path/to/2", endpoint2)
  ```
  
Verto also includes the option for named parameters in the path. Named parameters can 
be more strictly defined using regular expressions. Named parameters will be injected into  
`r.FormValue()` and, if the endpoint is a `ResourceFunc`, will be retrievable through the  
[Context](#context) utility functions.  
  
  ```Go
    // Named routing example
    endpoint1 := verto.ResourceFunc(c *verto.Context) (interface{}, error) {
      fmt.Fprintf(c.Response, c.Get("param"))
    })
    endpoint2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      fmt.Fprintf(w, r.URL.Query().Get("param"))
    })
    
    // Named parameters are denoted by { }
    v.Add("GET", "/path/to/{param}", endpoint1)
    v.Add("POST", "/path/to/{param}", endpoint1)
    
    // Apply a regex check to the param by use of : followed by
    // the regex.
    v.AddHandler("PUT", "/path/to/{param: ^[0-9]+$}", endpoint2)
  ```
  
### Path redirection  
If a path contains extraneous symbols like extra /'s or .'s (barring trailing /'s), Verto will automatically
clean the path and send a redirect response to the cleaned path. By default, Verto will not attempt to redirect
paths with (without) trailing slashes if the other exists. Calling `SetStrict(false)` on a Verto instance
lets Verto know that you want it to redirect trailing slashes.
  
### Middleware Chaining  
  
Verto provides the option to chain middleware both globally and per route.
Middleware must come in one of 3 forms:  
- mux.PluginFunc `type PluginFunc(w http.ResponseWriter, r *http.Request, next http.Handler) func`  
- verto.PluginFunc `type PluginFunc(c *Context, next http.Handler) func`
- `http.Handler`
  
If the middleware comes in the form of an `http.Handler`, next is automatically called.  
  
 ```Go
    // Example middleware usage
    v := verto.New()

    // Simple plugins
    mw1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      fmt.Println("Hello, World 1!")
    })
    mw2 := mux.PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
      fmt.Println("Hello, World 2!")
      next(w, r)
    })
    mw3 := verto.PluginFunc(func(c *verto.Context, next http.Handler) {
      fmt.Println("Hello, World 3!")
      next(w, r)
    })
    
    // Register global plugins: These will run for every single
    // request. Middleware is served first come - first serve.
    v.UseHandler(mw1)
    v.Use(mw2)
    v.UseVerto(mw3)
    
    // Register route
    v.Add("GET", "/", func(c *verto.Context) (interface{}, error) {
      fmt.Fprintf(c.Response, "Finished.")
    })
    
    v.Run()
  ```
    
Middleware can also be chained per route  
  
  ```Go
    // Register route and chain middle ware. These middleware
    // will only be run for GET requests on /.
    v.Add("GET", "/", func(c *verto.Context) (interface{}, error) {
      ...
    }).UseHandler(mw1).Use(mw2).UseVerto(mw3)
  ```
  
### Groups
  
Verto provides the ability to create route groups.  

  ```Go
  // Create a group
  g := v.Group("GET", "/path/path2/path3")
  
  // Add endpoint handlers to a route group. The full path
  // for the handler will be /path/path2/path3/handler. 
  g.Add("GET", "/handler", http.HandlerFunc(
    func(w http.ResponseWriter, http.Request) {
      fmt.Fprintf(w, "Hello!")
    },
  ))
  
  // Middleware can be chained per route group. Middleware
  // will be run for all subgroups and handlers under the parent
  // group.
  g.Use(mux.PluginFunc(
    func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
      fmt.Fprintf(w, "World!")
    },
  ))
  
  // Create subgroups attached to a group. The resulting subgroup
  // path will be /path/path2/path3/path4.
  sub := g.Group("/path4")
  ```
  Creating a group when there exists other groups/endpoint handlers that share
  a path prefix with the new group will cause those groups/endpoints to be subsumed
  under the new group. When creating groups, all wildcard segments are treated as
  equal. Thus, groups/endpoints subsumed under a new group with a wildcard segment
  in the shared prefix will use the new group's wildcard segment as key instead of
  their old segments. Attempting to create an already existing group returns the
  existing group.  
    
### Context  
  
Context is the custom parameter used for Verto's `ResourceFunc`s.  
It contains the original `http.ResponseWriter` and `*http.Request`  
as well as a reference to a [logger](#logging) and [injections](#injections).  
Context also provides a set of utility functions for dealing with parameter  
setting and retrieval.  
  
  ```Go
    type Context struct {
      // The original ResponseWriter and Request
      Request  *http.Request
      Response http.ResponseWriter
      
      // Logger
      Logger Logger
      
      // Injections
      Injections *Injections
    }
    
    // Retrieves the first string value associated with the key. Throws an
    // error if the context was not properly initialized. If calling this function
    // on a Verto prepared context, an error will not be thrown. This goes for all
    // of the following functions
    func (c *Context) Get(key string) (string, error) { }
    
    // Performs exactly like Get() but returns all values associated with the key.
    func (c *Context) GetMulti(key string) ([]string, error) { }
    
    // Performs exactly like Get() but converts the value to a bool if possible.
    // Throws an error if the conversion fails or if the context was not properly
    // intialized.
    func (c *Context) GetBool(key string) (bool, error) { }
    
    // Does the same thing as GetBool() but converts to a float64 instead.
    func (c *Context) GetFloat64(key string) (float64, error) { }
    
    // Does the same thing as GetBool() but converts to an int64 instead.
    func (c *Context) GetInt64(key string) (int64, error) { }
    
    // Sets the value associated with key. Throws an error if context was not
    // properly initialized. 
    func (c *Context) Set(key, value string) error { }
    
    // Associated multiple values with the key. Throws an error if context
    // was not properly initialized.
    func (c *Context) SetMulti(key string, values []string) error { }
    
    // Associates a boolean value with the key. Throws an error if context
    // was not properly initialized or if there was a problem formatting value.
    func (c *Context) SetBool(key string, value bool) error { }
    
    // Associates a float64 value with the key. Throws an error if context
    // was not properly initialized or if there was a problem formatting value.
    func (c *Context) SetFloat64(key string, value float64) error { }
    
    // Associates a int64 value with the key. Throws an error if context
    // was not properly initialized or if there was a problem formatting value.
    func (c *Context) SetInt64(key string, value int64) error { }
  ```

**Note:** If you happen to be in the business of setting a large number of parameters,  
consider injecting a custom struct instead of using `Set()` or `SetMulti()`. While the `Set`  
functions are not super inefficient, they do use `url.Values.Encode()` and `url.Values.Decode()` 
  
### Response Handler  
  
Verto allows the user to define his own response handling function.  
The handler must be of the form:  
  
  ```Go
    type ResponseHandler interface {
      Handle(response interface{}, c *Context)
    }
  ```
  
Verto also defines a `ResponseFunc` to wrap functions so that  
they implement `ResponseHandler`. A default handler is provided  
and used if no custom handler is provided. It is recommended that  
the user brings his own handler as the default just attempts to  
write response as is.  
  
  ```Go
    // Custom response handler example
    handler := verto.ResponseFunc(func(response interface{}, c *verto.Context) {
      // Attempt to marshal response as JSON before writing it
      body, _ := json.Marshal(response)
      fmt.Fprintf(c.Response, body)
    })
    
    v.RegisterResponseHandler(handler)
  ```
  
### Error Handler  
  
Like response handling, Verto allows the user to define his own
error handler. The handler must be of the form:  
  
  ```Go
    type ErrorHandler interface {
      Handle(err error, c *Context)
    }
  ```
  
Verto also defines an `ErrorFunc` to wrap functions so that they implement  
`ErrorHandler`. A default handler is provided but it is recommended that  
the user brings his own handler. The default just responds with a `500 Internal Server Error`.  
  
  ```Go
    // Custom error handler example
    handler := verto.ErrorFunc(func(err error, c *Context) {
      fmt.Fprintf(c.Response, "Custom error response!")
    })
    
    v.RegisterErrorHandler(handler)
  ```
  
### Injections  
  
Injections are anything from the outside world you need passed to an endpoint  
handler. A single injection instance is passed to all handlers and plugins.  
**NOTE:** The `Set()`, `Get()`, `TryGet()`, `Delete()`, and `Clear()` functions **ARE** thread safe.  
  
  ```Go
    // Injection example
    
    // Inject using the Injections.Set() function. The first parameter
    // is the key used to access then injection from within
    // any handlers or plugins that implement verto.PluginFunc.
    v.Injections.Set("key", "value")
    
    // Inject anything like slices or structs
    sl := make([]int64, 5)
    st := &struct{
      Value int64,
    }{}
    v.Injections.Set("slice", sl)
    v.Injections.Set("struct", st)
    
    // Retrieve values with Get()
    st2 := v.Injections.Get("struct")
    
    // TryGet() lets you check if the value exists
    st3, exists := v.Injections.TryGet("struct")
    
    // Delete() removes an injection.
    v.Injections.Delete("slice")
    
    // Clear clears all injections
    v.Injections.Clear()
  ```
  
Injections are useful for things intricate loggers, analytics,  
and caching.  

### Logging  
  
Verto provides a default logger implementation but allows
custom loggers that implement the following interface:  
  
  ```Go
  type Logger interface {
    // The following functions print messages
    // at various levels to any open log files and subscribers
    Info(v ...interface{})
    Debug(v ...interface{})
    Warn(v ...interface{})
    Error(v ...interface{})
    Fatal(v ...interface{})
    Panic(v ...interface{})
    
    // The following functions print formatted messages
    // at various levels to any open log files and subscribers
    Infof(format string, v ...interface{})
    Debugf(format string, v ...interface{})
    Warnf(format string, v ...interface{}) 
    Errorf(format string, v ...interface{})
    Fatalf(format string, v ...interface{})
    Panicf(format string, v ...interface{})
    
    // Print a message to open log files and subscribers
    Print(v ...interface{})
    // Print a formatted message to open log files and subscribers
    Printf(format string, v ...interface{})
    
    // Close any open log files and subscriber channels. Returns
    // an error if there was any issue closing any files or channels.
    Close() error
  }
  ```

Custom logger implementations can be registered using the `RegisterLogger()` function.
