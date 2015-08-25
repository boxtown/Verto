package mux

import (
	"net/http"
	"testing"
)

func TestGroupAdd(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed group add"
	tVal := ""

	h1 := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			tVal = "A"
		},
	)
	h2 := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			tVal = "B"
		},
	)

	// Simple add
	pm := New()
	g1 := pm.Group("GET", "/path/to")
	g1.Add("/handler", h1)
	f, e := g1.(*group).matcher.match("/handler")
	if e != nil {
		t.Errorf(err)
	}
	if ep, ok := f.data().(*endpoint); !ok {
		t.Errorf(err)
	} else if ep.path != "/handler" {
		t.Errorf(err)
	}
	r, _ := http.NewRequest("GET", "http://test.com/path/to/handler", nil)
	pm.ServeHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}

	// Simple overwrite
	g1.Add("/handler", h2)
	pm.ServeHTTP(nil, r)
	if tVal != "B" {
		t.Errorf(err)
	}

	// Sub group add
	tVal = ""
	g1.Group("/another")
	g1.Add("/another/handler", h1)
	f, e = g1.(*group).matcher.match("/another/handler")
	if e != nil {
		t.Errorf(err)
	}
	if g, ok := f.data().(*group); !ok {
		t.Errorf(err)
	} else if g.path != "/another" {
		t.Errorf(err)
	}
	r, _ = http.NewRequest("GET", "http://test.com/path/to/another/handler", nil)
	pm.ServeHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
}

func TestGroupAddFunc(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed add func."
	pm := New()

	tVal := ""

	g1 := pm.Group("GET", "/path/to")
	g1.AddFunc("/handler", func(w http.ResponseWriter, r *http.Request) {
		tVal = "A"
	})
	f, _, _ := pm.find("GET", "/path/to/handler")
	r, _ := http.NewRequest("GET", "http://test.com/path/to/handler", nil)
	f.serveHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
}

func TestGroupGroup(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed group."
	g1 := newGroup("GET", "", nil)

	// test simple group
	tVal := ""
	g2 := g1.Group("/simple")
	g2.AddFunc("/handler", func(w http.ResponseWriter, r *http.Request) {
		tVal = "A"
	})
	r, _ := http.NewRequest("GET", "http://test.com/simple/handler", nil)
	g1.serveHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
	g3 := g1.Group("/simple")
	if g3 != g2 {
		t.Errorf(err)
	}

	tVal = ""
	g2 = g1.Group("/simple2/{wc}/simple2")
	g2.AddFunc("/handler", func(w http.ResponseWriter, r *http.Request) {
		tVal = "A"
	})
	r, _ = http.NewRequest("GET", "http://test.com/simple2/wctest/simple2/handler", nil)
	g1.serveHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
	g3 = g1.Group("/simple2/{wc}/simple2")
	if g3 != g2 {
		t.Errorf(err)
	}

	// Test subgroup
	tVal = ""
	g2 = g1.Group("/simple2/{wc}/simple2/simple2")
	g2.AddFunc("/handler", func(w http.ResponseWriter, r *http.Request) {
		tVal = "A"
	})
	r, _ = http.NewRequest("GET", "http://test.com/simple2/wctest/simple2/simple2/handler", nil)
	g1.serveHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}

	// Test subsume
	tVal = ""
	g2 = g1.Group("/simple2")
	r, _ = http.NewRequest("GET", "http://test.com/simple2/wctest/simple2/handler", nil)
	g1.serveHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}

	tVal = ""
	r, _ = http.NewRequest("GET", "http://test.com/simple2/wctest/simple2/simple2/handler", nil)
	g1.serveHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
}

func TestGroupPlugins(t *testing.T) {

	err := "Failed plugin test."

	tVal := ""
	tVal2 := ""
	tVal3 := ""

	// Test simple plugin chain
	pm := New()
	pm.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal = "A"
		next(w, r)
	}))

	g1 := pm.Group("GET", "/a/b")
	g1.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal2 = "B"
		next(w, r)
	}))

	ep := g1.AddFunc("/handler", func(w http.ResponseWriter, r *http.Request) {})
	ep.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal3 = "C"
		next(w, r)
	}))
	r, _ := http.NewRequest("GET", "http://test.com/a/b/handler", nil)
	pm.ServeHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
	if tVal2 != "B" {
		t.Errorf(err)
	}
	if tVal3 != "C" {
		t.Errorf(err)
	}

	// Test subgroup
	tVal = ""
	tVal2 = ""
	tVal3 = ""
	tVal4 := ""
	g2 := g1.Group("/c")
	g2.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal4 = "D"
		next(w, r)
	}))
	ep = g2.AddFunc("/handler", func(w http.ResponseWriter, r *http.Request) {})
	ep.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal3 = "E"
		next(w, r)
	}))
	r, _ = http.NewRequest("GET", "http://test.com/a/b/c/handler", nil)
	pm.ServeHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
	if tVal2 != "B" {
		t.Errorf(err)
	}
	if tVal3 != "E" {
		t.Errorf(err)
	}
	if tVal4 != "D" {
		t.Errorf(err)
	}

	// Test subsume
	tVal = ""
	tVal2 = ""
	tVal3 = ""
	tVal4 = ""
	tVal5 := ""
	g3 := pm.Group("GET", "/a")
	g3.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal5 = "F"
		next(w, r)
	}))
	r, _ = http.NewRequest("GET", "http://test.com/a/b/handler", nil)
	pm.ServeHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
	if tVal2 != "B" {
		t.Errorf(err)
	}
	if tVal3 != "C" {
		t.Errorf(err)
	}
	if tVal5 != "F" {
		t.Errorf(err)
	}
	r, _ = http.NewRequest("GET", "http://test.com/a/b/c/handler", nil)
	pm.ServeHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}
	if tVal2 != "B" {
		t.Errorf(err)
	}
	if tVal3 != "E" {
		t.Errorf(err)
	}
	if tVal4 != "D" {
		t.Errorf(err)
	}
	if tVal5 != "F" {
		t.Errorf(err)
	}
}
