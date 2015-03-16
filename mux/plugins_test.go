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

	p := newPlugins()
	p.use(h)
	if p.length != 1 {
		t.Errorf(err)
	}
	if p.head != p.tail {
		t.Errorf(err)
	}

	p.use(h2)
	if p.length != 2 {
		t.Errorf(err)
	}
	if p.head == p.tail {
		t.Errorf(err)
	}
	p.run(nil, nil)

	if tVal != "A" {
		t.Errorf(err)
	}

	if tVal2 != "B" {
		t.Errorf(err)
	}
}

func TestPluginsPopHead(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed pop head."
	p := newPlugins()

	// Test empty pop
	p.popHead()
	if p.head != emptyPlugin {
		t.Errorf(err)
	}
	if p.tail != emptyPlugin {
		t.Errorf(err)
	}

	h := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {})

	// Test pop one
	p.use(h)
	p.popHead()
	if p.head != emptyPlugin {
		t.Errorf(err)
	}
	if p.tail != emptyPlugin {
		t.Errorf(err)
	}

	// Test pop multiple
	p.use(h)
	p.use(h)
	p.popHead()
	if p.head == emptyPlugin {
		t.Errorf(err)
	}
	if p.tail == emptyPlugin {
		t.Errorf(err)
	}
}

func TestPluginsPopTail(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed pop tail."
	p := newPlugins()

	// Test empty pop
	p.popTail()

	h := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {})

	// Test pop one
	p.use(h)
	p.popTail()
	if p.head != emptyPlugin {
		t.Errorf(err)
	}
	if p.tail != emptyPlugin {
		t.Errorf(err)
	}
	if p.length != 0 {
		t.Errorf(err)
	}
}
