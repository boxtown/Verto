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
	cType() cType
	exec(w http.ResponseWriter, r *http.Request)
	join(parent *group)
}
