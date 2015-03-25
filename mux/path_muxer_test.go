// path_muxer_test
package mux

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPathMuxerFind(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed find."
	pm := New()
	m := newMuxNode(nil, "")

	// Test basic find
	pm.nodes.Add("/path/to/handler", m)
	node, _, _, _ := pm.find("/path/to/handler")
	if node != m {
		t.Errorf(err)
	}

	// Test not found
	_, _, _, e := pm.find("/path/to/nf")
	if e != ErrNotFound {
		t.Errorf(e.Error())
	}

	// Test redirect
	_, _, _, e = pm.find("/path/to/handler/")
	if e != ErrRedirectSlash {
		t.Errorf(e.Error())
	}

	// Test basic subgroup find
	sub := New()
	sub.prefix = "/path2"
	sub.nodes.Add("/to/handler", m)
	pm.groups.Add("/path2", sub)
	node, _, _, _ = pm.find("/path2/to/handler")
	if node != m {
		t.Errorf(err)
	}

	// Test multi subgroup find
	sub = New()
	sub.prefix = "/path3"
	pm.groups.Add("/path3", sub)
	sub2 := New()
	sub2.prefix = "/path4"
	sub.groups.Add("/path4", sub2)
	sub2.nodes.Add("/to/handler", m)
	node, _, _, _ = pm.find("/path3/path4/to/handler")
	if node != m {
		t.Errorf(err)
	}

	// Test chaining
	tVal := ""
	tVal2 := ""
	tVal3 := ""
	pm.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal = "A"
		next(w, r)
	}))
	sub.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal2 = "B"
		next(w, r)
	}))
	sub2.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal3 = "C"
	}))
	_, _, chain, _ := pm.find("/path3/path4/to/handler")
	chain.run(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	}
	if tVal2 != "B" {
		t.Errorf(err)
	}
	if tVal3 != "C" {
		t.Errorf(err)
	}
}

func TestPathMuxerAdd(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed add."
	pm := New()

	tVal := ""

	// Test basic add
	pm.Add("GET", "/path/to/handler", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			tVal = "A"
		},
	))
	n, _, _ := pm.findNode("/path/to/handler", true)
	n.handlers["GET"].ServeHTTP(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	}

	// Test handler overwrite
	pm.Add("GET", "/path/to/handler", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			tVal = "B"
		},
	))
	n, _, _ = pm.findNode("/path/to/handler", true)
	n.handlers["GET"].ServeHTTP(nil, nil)
	if tVal != "B" {
		t.Errorf(err)
	}
}

func TestPathMuxerAddFunc(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed add func."
	pm := New()

	tVal := ""

	pm.AddFunc("GET", "/path/to/handler", func(w http.ResponseWriter, r *http.Request) {
		tVal = "A"
	})
	n, _, _ := pm.findNode("/path/to/handler", true)
	n.handlers["GET"].ServeHTTP(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	}
}

func TestPathMuxerUse(t *testing.T) {
	/* defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}() */

	err := "Failed use."
	pm := New()

	pm.AddFunc("GET", "/path/to/1", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if v := q.Get("local1"); v != "l1" {
			t.Errorf(err)
		}
		if v := q.Get("global1"); v != "g1" {
			t.Errorf(err)
		}
		if v := q.Get("global2"); v != "g2" {
			t.Errorf(err)
		}
	}).Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		q := r.URL.Query()
		if v := q.Get("global1"); v != "g1" {
			t.Errorf(err)
		}
		if v := q.Get("global2"); v != "g2" {
			t.Errorf(err)
		}

		q.Set("local1", "l1")
		r.URL.RawQuery = q.Encode()
		next(w, r)
	}))

	pm.AddFunc("GET", "/path/to/2", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if v := q.Get("local2"); v != "l2" {
			t.Errorf(err)
		}
		if v := q.Get("global1"); v != "g1" {
			t.Errorf(err)
		}
		if v := q.Get("global2"); v != "g2" {
			t.Errorf(err)
		}
	}).Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		q := r.URL.Query()
		if v := q.Get("global1"); v != "g1" {
			t.Errorf(err)
		}
		if v := q.Get("global2"); v != "g2" {
			t.Errorf(err)
		}

		q.Set("local2", "l2")
		r.URL.RawQuery = q.Encode()
		next(w, r)
	}))

	pm.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		q := r.URL.Query()
		q.Set("global1", "g1")
		r.URL.RawQuery = q.Encode()
		next(w, r)
	}))

	pm.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		q := r.URL.Query()
		if v := q.Get("global1"); v != "g1" {
			t.Errorf(err)
		}

		q.Set("global2", "g2")
		r.URL.RawQuery = q.Encode()
		next(w, r)
	}))

	r, _ := http.NewRequest("GET", "http://test.com/path/to/1", nil)
	pm.chain.run(nil, r)
	r, _ = http.NewRequest("GET", "http://test.com/path/to/2", nil)
	pm.chain.run(nil, r)
}

func TestPathMuxerUseHandler(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed use handler."
	pm := New()

	pm.AddFunc("GET", "/path/to/handler", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if v := q.Get("global"); v != "g" {
			t.Errorf(err)
		}
	})

	pm.UseHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		q.Set("global", "g")
		r.URL.RawQuery = q.Encode()
	}))

	r, _ := http.NewRequest("GET", "http://test.com/path/to/handler", nil)
	pm.chain.run(nil, r)
}

func TestNotFoundHandler(t *testing.T) {
	err := "Failed not found handler."

	w := httptest.NewRecorder()

	nfh := NotFoundHandler{}
	nfh.ServeHTTP(w, nil)

	if w.Body.String() != "Not Found." {
		t.Errorf(err)
	}
	if w.Code != 404 {
		t.Errorf(err)
	}
}

func TestNotImplementedHandler(t *testing.T) {
	err := "Failed not implemented handler."

	w := httptest.NewRecorder()

	nih := NotImplementedHandler{}
	nih.ServeHTTP(w, nil)

	if w.Body.String() != "Not Implemented." {
		t.Errorf(err)
	}
	if w.Code != 501 {
		t.Errorf(err)
	}
}

func TestRedirectHandler(t *testing.T) {
	err := "Failed not redirect handler."

	w := httptest.NewRecorder()

	r, _ := http.NewRequest("GET", "http://test.com", nil)

	rh := RedirectHandler{}
	rh.ServeHTTP(w, r)

	if w.Header().Get("Location") != "http://test.com" {
		t.Errorf(err)
	}
	if w.Code != 301 {
		t.Errorf(err)
	}
}

func TestPathMuxerServeHTTP(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed ServeHTTP."
	pm := New()

	tVal := ""

	pm.AddFunc("GET", "/path/to/handler", func(w http.ResponseWriter, r *http.Request) {
		tVal = "A"
	})

	w := httptest.NewRecorder()

	// Test successful request
	r, _ := http.NewRequest("GET", "http://test.com/path/to/handler", nil)
	pm.ServeHTTP(w, r)
	if tVal != "A" {
		t.Errorf(err)
	}

	// Test clean path
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "http://test.com/path/./to/../to/handler", nil)
	pm.ServeHTTP(w, r)
	if w.Code != 301 {
		t.Errorf(err)
	}
	if w.Header().Get("Location") != "http://test.com/path/to/handler" {
		t.Errorf(err)
	}

	// Test not found
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "http://test.com/nonexistent", nil)
	pm.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Errorf(err)
	}

	// Test not implemented
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("POST", "http://test.com/path/to/handler", nil)
	pm.ServeHTTP(w, r)
	if w.Code != 501 {
		t.Error(err)
	}

	// Test redirect
	w = httptest.NewRecorder()
	r, _ = http.NewRequest("GET", "http://test.com/path/to/handler/", nil)
	pm.Strict = false
	pm.ServeHTTP(w, r)
	if w.Code != 301 {
		t.Errorf(err)
	}
	if w.Header().Get("Location") != "http://test.com/path/to/handler" {
		t.Errorf(err)
	}
}
