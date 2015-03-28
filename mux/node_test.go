package mux

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewNodeImpl(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed test newNodeImpl."
	n := newMuxNode(nil, "")
	tVal := ""
	tVal2 := ""

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tVal = "A"
	})
	p := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal2 = "B"
	})

	n.handlers["GET"] = handler
	n.chains["POST"] = newPlugins()
	n.chains["POST"].use(p)

	ni := newNodeImpl("GET", n)
	if ni.method != "GET" {
		t.Errorf(err)
	}
	ni.handlers["GET"].ServeHTTP(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	}
	ni.chains["POST"].run(nil, nil)
	if tVal2 != "B" {
		t.Errorf(err)
	}
}

func TestNodeImplUse(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed nodeImpl use."
	n := newMuxNode(nil, "")
	tVal := ""
	tVal2 := ""

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tVal = "A"
	})
	p := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal2 = "B"
		next(w, r)
	})

	n.handlers["GET"] = handler
	ni := newNodeImpl("GET", n)
	ni.Use(p)

	r, _ := http.NewRequest("GET", "", nil)
	n.chains["GET"].run(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
	if tVal2 != "B" {
		t.Errorf(err)
	}
}

func TestNodeServeHTTP(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed node ServeHTTP."
	n := newMuxNode(New(), "")
	tVal := ""
	tVal2 := ""
	tVal3 := ""

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tVal = "A"
	})
	handler2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tVal2 = "B"
	})

	p := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal3 = "C"
		next(w, r)
	})

	n.handlers["GET"] = handler
	n.handlers["POST"] = handler2

	// Test non-existent method
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("PUT", "", nil)
	n.ServeHTTP(w, r)
	if w.Code != 501 {
		t.Errorf(err)
	}

	// Test different methods
	r, _ = http.NewRequest("GET", "", nil)
	n.ServeHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
	r, _ = http.NewRequest("POST", "", nil)
	n.ServeHTTP(nil, r)
	if tVal2 != "B" {
		t.Errorf(err)
	}

	// Test run chain
	ni := newNodeImpl("GET", n)
	ni.Use(p)
	tVal = ""
	r, _ = http.NewRequest("GET", "", nil)
	n.ServeHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
	if tVal3 != "C" {
		t.Errorf(err)
	}
}
