package mux

import (
	"net/http"
)

// -----------------------------
// ---------- Plugins ----------

// PluginHandler is an interface for Plugins.
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

// Plugin implements the http.Handler interface. It is a linked list
// of plugins.
type plugin struct {
	handler PluginHandler
	next    *plugin
	prev    *plugin
}

var emptyPlugin = &plugin{
	handler: PluginFunc(func(w http.ResponseWriter, r *http.Request, nex http.HandlerFunc) {

	}),
}

func (p *plugin) run(w http.ResponseWriter, r *http.Request) {
	p.handler.Handle(w, r, p.next.run)
}

type plugins struct {
	head *plugin
	tail *plugin

	length int64
}

func newPlugins() *plugins {
	return &plugins{emptyPlugin, emptyPlugin, 0}
}

func (p *plugins) deepCopy() *plugins {
	cpy := newPlugins()
	next := p.head
	for next != nil && next != emptyPlugin {
		cpy.use(next.handler)
		next = next.next
	}
	return cpy
}

func (p *plugins) use(handler PluginHandler) {
	p.length = p.length + 1

	plugin := &plugin{
		handler: handler,
		next:    emptyPlugin,
		prev:    emptyPlugin,
	}

	if p.head == nil || p.head == emptyPlugin {
		p.head = plugin
		p.tail = p.head
		return
	}

	p.tail.next = plugin
	plugin.prev = p.tail
	p.tail = plugin
}

func (p *plugins) popHead() {
	if p.head == nil || p.head == emptyPlugin {
		return
	}

	p.length = p.length - 1

	p.head = p.head.next
	if p.head != nil && p.head != emptyPlugin {
		p.head.prev = emptyPlugin
	} else {
		p.tail = emptyPlugin
	}
}

func (p *plugins) popTail() {
	if p.tail == nil || p.tail == emptyPlugin {
		return
	}

	p.length = p.length - 1

	p.tail = p.tail.prev
	if p.tail != nil && p.tail != emptyPlugin {
		p.tail.next = emptyPlugin
	} else {
		p.head = emptyPlugin
	}
}

func (p *plugins) run(w http.ResponseWriter, r *http.Request) {
	if p.head == nil || p.head == emptyPlugin {
		return
	}

	p.head.run(w, r)
}
