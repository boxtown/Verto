package mux

import (
	"net/http"
)

type cType int

const (
	GROUP    cType = 0
	ENDPOINT cType = 1
)

type compilable interface {
	compile()
	join(parent *group)
	serveHTTP(w http.ResponseWriter, r *http.Request)
	cType() cType
}
