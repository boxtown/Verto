# Verto
  
Verto is a simple REST framework that provides routing, error handling, response handling,  
logging, and middleware chaining.  
  
  - [Basic Usage](#basic-usage)
  - [Resource Function](#resource-function)
  - [Routing](#routing)
  - [Middleware Chaining](#middleware-chaining)
  - [Context](#context)
  - [Response Handler](#response-handler)
  - [Error Handler](#error-handler)
  - [Injections](#injections)
  - [Logging](#logging)
  
### Basic Usage  
  
    // Initialize Verto
    v := verto.New()
    
    // Register a route
    v.Register("GET", "/", func(c *verto.Context) (interface{}, error) {
      fmt.Fprintf(c.Response, "Hello, World!")
    })
    
    // Run Verto
    v.Run()
  
### Resource Function
  
Verto accepts a custom ResourceFunction:  
  
    type ResourceFunc(c *Context) (interface{}, error) func
  
The function takes in a Verto [Context](#context) struct. The Context contains  
a `map` of [injections](#injections) that we will discuss later. The resource function  
returns an `interface{}` and optionally an `error`. If the call to the resource was successful,  
a nil `error` should be returned. The default Verto instance will be able to handle any `error`s  
or `interface{}` return values but in a very basic way. It is recommended for the user to bring  
a custom [response](#response-handler) and [error](#error-handler) handler to Verto.  
  
Verto will also happily take `http.Handler` as a routing endpoint.  
  
### Routing  
  
Verto offers simple routing. Endpoints are registered using the `Register` and `RegisterHandler`  
methods. The parameters are the method to be matched, the path to be matched, and the endpoint  
handler. `Register` takes a `verto.ResourceFunc` as an endpoint and `RegisterHandler` takes a  
normal `http.Handler`. 
  
    // Basic routing example
    endpoint1 := verto.ResourceFunc(c *verto.Context) (interface{}, error) {
      fmt.Fprintf(c.Response, "Hello, World!)
    })
    endpoint2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      fmt.Fprintf(w, "Hello, World!)
    })
    
    v.Register("GET", "/path/to/1", endpoint1)
    v.Register("POST", "/path/to/1", endpoint1)
    
    v.RegisterHandler("PUT", "/path/to/2", endpoint2)
  
Verto also the option to include named parameters in the path. Named parameters can also
be more strictly defined using regular expressions. Named parameters will be injected into  
`r.URL.Query()` and, if the endpoint is a `ResourceFunc`, into `c.Params` as well.  
  
    // Named routing example
    endpoint1 := verto.ResourceFunc(c *verto.Context) (interface{}, error) {
      fmt.Fprintf(c.Response, c.Params.Get("param"))
    })
    endpoint2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      fmt.Fprintf(w, r.URL.Query().Get("param"))
    })
    
    // Named parameters are denoted by { }
    v.Register("GET", "/path/to/{param}", endpoint1)
    v.Register("POST", "/path/to/{param}", endpoint1)
    
    // Apply a regex check to the param by use of : followed by
    // the regex.
    v.RegisterHandler("PUT", "/path/to/{param: ^[0-9]+$}", endpoint2)
  
  
### Middleware Chaining  
  
Verto provides the option to chain middleware both globally and per route.
Middleware must come in one of 3 forms:  
- mux.PluginFunc `type PluginFunc(w http.ResponseWriter, r *http.Request, next http.Handler) func`  
- verto.PluginFunc `type PluginFunc(c *Context, next http.Handler) func`
- `http.Handler`
  
If the middleware comes in the form of an `http.Handler`, next is automatically called.  
  
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
    v.Register("GET", "/", func(c *verto.Context) (interface{}, error) {
      fmt.Fprintf(c.Response, "Finished.")
    })
    
    v.Run()
    
Middleware can also be chained per route  
  
    // Register route and chain middle ware. These middleware
    // will only be run for GET requests on /.
    v.Register("GET", "/", func(c *verto.Context) (interface{}, error) {
      ...
    }).UseHandler(mw1).Use(mw2).UseVerto(mw3)
  
### Context  
  
Context is the custom parameter used for Verto's `ResourceFunc`s.  
It contains the original `http.ResponseWriter` and `*http.Request`  
as well as a reference to a [logger](#logging) and [injections](#injections).  
Context also provides a set of utility functions for dealing with parameter  
setting and retrieval.  
  
    type Context struct {
      // The original ResponseWriter and Request
      Request  *http.Request
      Response http.ResponseWriter
      
      // Logger
      Logger Logger
      
      // Injections
      Injections map[string]interface{}
    }
    
    // Retrieves the first string value associated with the key. Throws an
    // error if the context was not properly initialized. If calling this function
    // on a Verto prepared context, an error will not be thrown. This goes for all
    // of the following functions
    func (c *Context) Get(key string) (string, error)
    
    // Performs exactly like Get() but returns all values associated with the key.
    func (c *Context) GetMulti(key string) ([]string, error)
    
    // Performs exactly like Get() but converts the value to a bool if possible.
    // Throws an error if the conversion fails or if the context was not properly
    // intialized.
    func (c *Context) GetBool(key string) (bool, error)
    
    // Does the same thing as GetBool() but converts to a float64 instead.
    func (c *Context) GetFloat64(key string) (float64, error)
    
    // Does the same thing as GetBool() but converts to an int64 instead.
    func (c *Context) GetInt64(key string) (int64, error)
    
    // Sets the value associated with key. Throws an error if context was not
    // properly initialized. 
    func (c *Context) Set(key, value string) error
    
    // Associated multiple values with the key. Throws an error if context
    // was not properly initialized.
    func (c *Context) SetMulti(key string, values []string) error
    
    // Associates a boolean value with the key. Throws an error if context
    // was not properly initialized or if there was a problem formatting value.
    func (c *Context) SetBool(key string, value bool) error
    
    // Associates a float64 value with the key. Throws an error if context
    // was not properly initialized or if there was a problem formatting value.
    func (c *Context) SetFloat64(key string, value float64) error
    
    // Associates a int64 value with the key. Throws an error if context
    // was not properly initialized or if there was a problem formatting value.
    func (c *Context) SetInt64(key string, value int64) error
  
### Response Handler  
  
Verto allows the user to define his own response handling function.  
The handler must be of the form:  
  
    type ResponseHandler interface {
      Handle(response interface{}, c *Context)
    }
  
Verto also defines a `ResponseFunc` to wrap functions so that  
they implement `ResponseHandler`. A default handler is provided  
and used if no custom handler is provided. It is recommended that  
the user brings his own handler as the default just attempts to  
write response as is.  
  
    // Custom response handler example
    handler := verto.ResponseFunc(func(response interface{}, c *verto.Context) {
      // Attempt to marshal response as JSON before writing it
      body, _ := json.Marshal(response)
      fmt.Fprintf(c.Response, body)
    })
    
    v.RegisterResponseHandler(handler)
  
### Error Handler  
  
Like response handling, Verto allows the user to define his own
error handler. The handler must be of the form:  
  
    type ErrorHandler interface {
      Handle(err error, c *Context)
    }
  
Verto also defines an `ErrorFunc` to wrap functions so that they implement  
`ErrorHandler`. A default handler is provided but it is recommended that  
the user brings his own handler. The default just responds with a `500 Internal Server Error`.  
  
    // Custom error handler example
    handler := verto.ErrorFunc(func(err error, c *Context) {
      fmt.Fprintf(c.Response, "Custom error response!")
    })
    
    v.RegisterErrorHandler(handler)
  
### Injections  
  
Injections are anything from the outside world you need passed to an endpoint  
handler. 
  
    // Injection example
    
    // Inject using the Inject() function. The first parameter
    // is the key used to access then injection from within
    // any handlers or plugins that implement verto.PluginFunc.
    v.Inject("key", "value")
    
    // Inject anything like slices or structs
    sl := make([]int64, 5)
    st := &struct{
      Value int64,
    }{}
    v.Inject("slice", sl)
    v.Inject("struct", st)
  
Injections are useful for things intricate loggers, analytics,  
and caching.  

### Logging  
  
Verto supports the use of logging but does not provide a default  
option. A custom logger can be registered as long as it follows  
the form:  
  
    type Logger interface {
     // Log logs message msg to the default location
     // and returns true on a successful log.
     Log(msg string) bool
     
     // LogTo logs message msg to the location identified
     // by name. Returns true on a successful log.
     LogTo(name, msg string) bool
   
Example:  
  
    type LogWrapper struct {
      log.Logger
    }
    
    func (lw LogWrapper) Log(msg string) bool {
      lw.Logger.Print(msg)
      return true
    }
    
    func (lw LogWrapper) LogTo(name, msg string) bool {
      lw.Logger.Print(msg)
      return true
    }
    
    f, _ := os.Open("/path/to/logfile.log")
    l, _ := log.New(f, "Prefix: ", 0)
    
    v.RegisterLogger(&LogWrapper{l})
    // flag must be set to log
    v.SetLogging(true)
