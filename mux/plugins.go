package mux

import (
	"net/http"
)

// -----------------------------
// ---------- Plugins ----------

// PluginHandler is an interface for Plugins.
// If the Plugin ran successfully, call next
// to continue the chain of Plugins.
type PluginHandler interface {
	Handle(w http.ResponseWriter, r *http.Request, next http.HandlerFunc)
}

// PluginFunc wraps functions so that they implement the PluginHandler interface.
type PluginFunc func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc)

// Handle calls the function wrapped as a PluginFunc.
func (p PluginFunc) Handle(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	p(w, r, next)
}

// Plugin implements the http.Handler interface. It is a linked list
// of Plugins.
type Plugin struct {
	handler PluginHandler
	next    *Plugin
	prev    *Plugin
}

// EmptyPlugin represents an empty Plugin with a no-op handler
var EmptyPlugin = &Plugin{
	handler: PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	}),
}

// Run calls the Plugin's handler passing it the next Plugin in line
func (p *Plugin) Run(w http.ResponseWriter, r *http.Request) {
	p.handler.Handle(w, r, p.next.Run)
}

// Plugins is a doubly-linked list of Plugins
type Plugins struct {
	head   *Plugin
	tail   *Plugin
	length int
}

// Returns a newly initialized Plugins with head and tail set
// to the EmptyPlugin
func NewPlugins() *Plugins {
	return &Plugins{EmptyPlugin, EmptyPlugin, 0}
}

// DeepCopy returns a deepy copy of Plugins that is
// safe for manipulation
func (p *Plugins) DeepCopy() *Plugins {
	cpy := NewPlugins()
	next := p.head
	for next != nil && next != EmptyPlugin {
		cpy.Use(next.handler)
		next = next.next
	}
	return cpy
}

func (p *Plugins) Length() int {
	return p.length
}

// Link links p2 onto the end of this Plugins
func (p *Plugins) Link(p2 *Plugins) {
	if p2 == nil || p2.head == EmptyPlugin {
		return
	}
	if p.head == EmptyPlugin {
		p.head = p2.head
		p.tail = p2.tail
		p.length = p2.length
		return
	}

	p.tail.next = p2.head
	p2.head.prev = p.tail
	p.tail = p2.tail
	p.length += p2.length
}

// Use appends handler onto the end of the chain
// of plugins represented by Plugins
func (p *Plugins) Use(handler PluginHandler) {
	p.length = p.length + 1

	Plugin := &Plugin{
		handler: handler,
		next:    EmptyPlugin,
		prev:    EmptyPlugin,
	}

	if p.head == EmptyPlugin {
		p.head = Plugin
		p.tail = p.head
		return
	}

	p.tail.next = Plugin
	Plugin.prev = p.tail
	p.tail = Plugin
}

// Run runs all the Plugins in Plugins in the order they were added.
func (p *Plugins) Run(w http.ResponseWriter, r *http.Request) {
	if p.head == EmptyPlugin {
		return
	}

	p.head.Run(w, r)
}
