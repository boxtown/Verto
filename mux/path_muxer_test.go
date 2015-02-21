// path_muxer_test
package mux

import (
	"net/http"
	"testing"
)

func TestPluginRun(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed to run."
	tVal := ""
	tVal2 := ""

	h := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal = "A"
		next(w, r)
	})

	h2 := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal2 = "B"
	})

	p := &plugin{}
	p2 := &plugin{}

	p.handler = h
	p2.handler = h2
	p.next = p2

	p.run(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	} else if tVal2 != "B" {
		t.Errorf(err)
	}
}

func TestPluginsUse(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed to use."
	tVal := ""
	tVal2 := ""

	h := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal = "A"
		next(w, r)
	})

	h2 := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal2 = "B"
	})

	p := &plugins{}
	p.use(h)
	if p.length != 1 {
		t.Errorf(err)
	}

	p.use(h2)
	if p.length != 2 {
		t.Errorf(err)
	}
	p.run(nil, nil)

	if tVal2 != "B" {
		t.Errorf(err)
	}
}
