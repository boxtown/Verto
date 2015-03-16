package mux

import (
	"net/http"
	"testing"
)

func TestMuxNodeUse(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed mux node use."
	tVal := ""
	tVal2 := ""
	tVal3 := ""

	m := newMuxNode(nil, "")
	m.handlers["GET"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tVal = "end"
	})

	// Test use one
	m.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal2 = "A"
		next(w, r)
	}))
	r, _ := http.NewRequest("GET", "", nil)
	m.chain.run(nil, r)

	if tVal != "end" {
		t.Errorf(err)
	}
	if tVal2 != "A" {
		t.Errorf(err)
	}

	tVal = ""
	tVal2 = ""

	// Test use multiple
	m.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal3 = "B"
		next(w, r)
	}))
	m.chain.run(nil, r)

	if tVal != "end" {
		t.Errorf(err)
	}
	if tVal2 != "A" {
		t.Errorf(err)
	}
	if tVal3 != "B" {
		t.Errorf(err)
	}

	// Test ordering of plugins
	m = newMuxNode(nil, "")
	m.handlers["GET"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if v := q.Get("one"); v != "A" {
			t.Errorf(err)
		}

		if v := q.Get("two"); v != "B" {
			t.Errorf(err)
		}
	})

	m.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		q := r.URL.Query()
		q.Set("one", "A")
		r.URL.RawQuery = q.Encode()
		next(w, r)
	}))

	m.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		q := r.URL.Query()
		if v := q.Get("one"); v != "A" {
			t.Errorf(err)
		}

		q.Set("two", "B")
		r.URL.RawQuery = q.Encode()
		next(w, r)
	}))
	m.chain.run(nil, r)
}

func TestMuxNodeUseHandler(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed use handler."
	tVal := ""
	tVal2 := ""

	m := newMuxNode(nil, "")
	m.handlers["GET"] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tVal = "end"
	})
	m.UseHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tVal2 = "A"
	}))

	r, _ := http.NewRequest("GET", "", nil)
	m.chain.run(nil, r)

	if tVal != "end" {
		t.Errorf(err)
	}
	if tVal2 != "A" {
		t.Errorf(err)
	}
}
