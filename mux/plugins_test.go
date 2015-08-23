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

	p := &Plugin{}
	p2 := &Plugin{}

	p.handler = h
	p2.handler = h2
	p.next = p2

	p.Run(nil, nil)
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

	p := NewPlugins()
	p.Use(h)
	if p.length != 1 {
		t.Errorf(err)
	}
	if p.head != p.tail {
		t.Errorf(err)
	}

	p.Use(h2)
	if p.length != 2 {
		t.Errorf(err)
	}
	if p.head == p.tail {
		t.Errorf(err)
	}
	p.Run(nil, nil)

	if tVal != "A" {
		t.Errorf(err)
	}

	if tVal2 != "B" {
		t.Errorf(err)
	}
}

func TestPluginsDeepCopy(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed deep copy."
	tVal := ""

	h := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal = "A"
		next(w, r)
	})

	h2 := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal = "B"
		next(w, r)
	})

	h3 := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal = "C"
	})

	p := NewPlugins()
	p2 := p.DeepCopy()

	// Test Blank run
	p2.Run(nil, nil)

	p.Use(h)

	// Test copy one
	p2 = p.DeepCopy()
	if p2.length != 1 {
		t.Errorf(err)
	}
	p2.Run(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	}

	// Test copy multiple
	p.Use(h2)
	p2 = p.DeepCopy()
	if p2.length != 2 {
		t.Errorf(err)
	}
	p2.Run(nil, nil)
	if tVal != "B" {
		t.Errorf(err)
	}

	// Test uniqueness
	tVal = ""
	p2.Use(h3)
	p.Run(nil, nil)
	if tVal != "B" {
		t.Errorf(err)
	}
	p2.Run(nil, nil)
	if tVal != "C" {
		t.Errorf(err)
	}
}

func TestPluginsLink(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed link."
	tVal := ""

	h := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal = "A"
		next(w, r)
	})

	h2 := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal = "B"
		next(w, r)
	})

	h3 := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal = "C"
		next(w, r)
	})

	h4 := PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal = "D"
	})

	p := NewPlugins()
	p2 := NewPlugins()

	// link empty
	p.Link(p2)
	if p.length != 0 {
		t.Errorf(err)
	}

	// link empty to one
	p2.Use(h)
	p.Link(p2)
	if p.length != 1 {
		t.Errorf(err)
	}
	p.Run(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	}

	// link one to empty
	tVal = ""
	p = NewPlugins()
	p2.Link(p)
	if p2.length != 1 {
		t.Errorf(err)
	}
	p2.Run(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	}

	// link one to one
	tVal = ""
	p = NewPlugins()
	p2 = NewPlugins()
	p.Use(h)
	p2.Use(h2)
	p.Link(p2)
	if p.length != 2 {
		t.Errorf(err)
	}
	p.Run(nil, nil)
	if tVal != "B" {
		t.Errorf(err)
	}

	// link many to many
	tVal = ""
	p = NewPlugins()
	p2 = NewPlugins()
	p.Use(h)
	p.Use(h2)
	p2.Use(h3)
	p2.Use(h4)
	p.Link(p2)
	if p.length != 4 {
		t.Errorf(err)
	}
	p.Run(nil, nil)
	if tVal != "D" {
		t.Errorf(err)
	}
}
