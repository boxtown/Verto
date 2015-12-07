// plugins is package providing a number of common middleware plugins
// for the Verto framework. Currently included are plugins for
// compression handling, panic recovery, and CORS handling
package plugins

import (
	"github.com/boxtown/verto"
	"net/http"
)

// PluginCore represents core functionality for
// all Verto pre-built plugins
type Core struct {
	// Verbose is flag to determine the verbosity
	// of a plugin
	Verbose bool

	// OnEnter is an optional callback to run each
	// time the plugin enters execution
	OnEnter func(c *verto.Context)

	// OnExit is an optional callback to run each
	// time the plugin exits execution
	OnExit func(c *verto.Context)

	// Id is an id for the plugin
	Id string
}

// Handle wraps a plugin function within Core plugin
// functionality. This allows the OnEnter and OnExit
// functions to run for the wrapped plugin
func (core Core) Handle(
	f func(*verto.Context, http.HandlerFunc),
	c *verto.Context,
	next http.HandlerFunc) {

	if core.Verbose {
		c.Logger.Infof("Entering %s...", core.Id)
		defer c.Logger.Infof("Exiting %s...", core.Id)
	}
	if core.OnEnter != nil {
		core.OnEnter(c)
	}
	if core.OnExit != nil {
		defer core.OnExit(c)
	}
	f(c, next)
}
