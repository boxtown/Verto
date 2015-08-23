package mux

import (
	"net/http"
)

type CType int

const (
	GROUP    CType = 0
	ENDPOINT CType = 1
)

type Compilable interface {
	Compile()
	Join(parent *group)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	Type() CType
}
