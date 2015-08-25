package mux

import (
	"net/http"
)

// -----------------------------
// ---------- plugins ----------

// PluginHandler is an interface for plugins.
// If the plugin ran successfully, call next
// to continue the chain of plugins.
type PluginHandler interface {
	Handle(w http.ResponseWriter, r *http.Request, next http.HandlerFunc)
}

// PluginFunc wraps functions so that they implement the PluginHandler interface.
type PluginFunc func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc)

// Handle calls the function wrapped as a PluginFunc.
func (p PluginFunc) Handle(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	p(w, r, next)
}

// plugin implements the http.Handler interface. It is a linked list
// of plugins.
type plugin struct {
	handler PluginHandler
	next    *plugin
	prev    *plugin
}

// emptyPlugin represents an empty plugin with a no-op handler
var emptyPlugin = &plugin{
	handler: PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	}),
}

// Run calls the plugin's handler passing it the next plugin in line
func (p *plugin) run(w http.ResponseWriter, r *http.Request) {
	p.handler.Handle(w, r, p.next.run)
}

// plugins is a doubly-linked list of plugins
type plugins struct {
	head   *plugin
	tail   *plugin
	length int
}

// Returns a newly initialized plugins with head and tail set
// to the emptyPlugin
func newPlugins() *plugins {
	return &plugins{emptyPlugin, emptyPlugin, 0}
}

// DeepCopy returns a deepy copy of plugins that is
// safe for manipulation
func (p *plugins) deepCopy() *plugins {
	cpy := newPlugins()
	next := p.head
	for next != nil && next != emptyPlugin {
		cpy.use(next.handler)
		next = next.next
	}
	return cpy
}

// Link links p2 onto the end of this plugins
func (p *plugins) link(p2 *plugins) {
	if p2 == nil || p2.head == emptyPlugin {
		return
	}
	if p.head == emptyPlugin {
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
// of plugins represented by plugins
func (p *plugins) use(handler PluginHandler) {
	p.length = p.length + 1

	plugin := &plugin{
		handler: handler,
		next:    emptyPlugin,
		prev:    emptyPlugin,
	}

	if p.head == emptyPlugin {
		p.head = plugin
		p.tail = p.head
		return
	}

	p.tail.next = plugin
	plugin.prev = p.tail
	p.tail = plugin
}

// Run runs all the plugins in plugins in the order they were added.
func (p *plugins) run(w http.ResponseWriter, r *http.Request) {
	if p.head == emptyPlugin {
		return
	}

	p.head.run(w, r)
}
