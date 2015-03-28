package mux

import (
	"net/http"
)

// Group is an interface for interacting with route groups
// in mux.
type Group interface {
	Add(method, path string, handler http.Handler) Node
	AddFunc(method, path string, f func(w http.ResponseWriter, r *http.Request)) Node
	Group(path string) Group
	Use(handler PluginHandler) Group
	UseHandler(handler http.Handler) Group
}
