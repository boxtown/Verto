// path_muxer_test
package mux

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrimPathPrefix(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Fail trim path prefix"

	path := ""
	prefix := ""
	x := trimPathPrefix(path, prefix, false)
	if x != "" {
		t.Errorf(err)
	}

	path = "/"
	x = trimPathPrefix(path, prefix, false)
	if x != "/" {
		t.Errorf(err)
	}

	prefix = "/"
	x = trimPathPrefix(path, prefix, false)
	if x != "" {
		t.Errorf(err)
	}

	path = "/a/b"
	prefix = "/a/"
	x = trimPathPrefix(path, prefix, false)
	if x != "b" {
		t.Errorf(err)
	}

	path = "/a/b/c/d/e"
	prefix = "/a/b/d/e"
	x = trimPathPrefix(path, prefix, false)
	if x != path {
		t.Errorf(err)
	}

	path = "{a}"
	prefix = "{b}"
	x = trimPathPrefix(path, prefix, false)
	if x != "" {
		t.Errorf(err)
	}

	path = "{a}/b/c/{d}/e/{f}"
	prefix = "{b}/b/c/{e}/e/{g}"
	x = trimPathPrefix(path, prefix, false)
	if x != "" {
		t.Errorf(err)
	}
	prefix = "b/b/c/{d}/e/{f}"
	x = trimPathPrefix(path, prefix, false)
	if x != path {
		t.Errorf(err)
	}
	prefix = "{b}/b/c/{e}/e"
	x = trimPathPrefix(path, prefix, false)
	if x != "/{f}" {
		t.Errorf(err)
	}
	path = "b/b/c/{d}/e/{f}"
	prefix = "{b}/b/c/{d}/e/{f}"
	x = trimPathPrefix(path, prefix, true)
	if x != "" {
		t.Errorf(x)
	}
}

func TestPathsEqual(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Fail paths equal"

	p1 := ""
	p2 := ""
	if !pathsEqual(p1, p2) {
		t.Errorf(err)
	}

	p2 = "/"
	if pathsEqual(p1, p2) {
		t.Errorf(err)
	}

	p1 = "/a/b/c"
	p2 = "/a/b/c"
	if !pathsEqual(p1, p2) {
		t.Errorf(err)
	}

	p2 = "{a}/b/c"
	if pathsEqual(p1, p2) {
		t.Errorf(err)
	}
	p1 = "{b}/b/c"
	if !pathsEqual(p1, p2) {
		t.Errorf(err)
	}
	p1 = "{b}/b/{c}/d"
	p2 = "{a}/b/{d}/d"
	if !pathsEqual(p1, p2) {
		t.Errorf(err)
	}
	p2 = "{a}/b/c/d"
	if pathsEqual(p1, p2) {
		t.Error(err)
	}
	p1 = "{b}/a/b/{c}"
	p2 = "{a}/a/b/{d}"
	if !pathsEqual(p1, p2) {
		t.Errorf(err)
	}
	p2 = "{a}/a/b/c"
	if pathsEqual(p1, p2) {
		t.Errorf(err)
	}
	p2 = "{a}/a/c/{d}"
	if pathsEqual(p1, p2) {
		t.Errorf(err)
	}

}

func TestPathMuxerFind(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed find."
	pm := New()
	m := &matcher{}
	pm.matchers["GET"] = m
	ep := &endpoint{}
	var v []param

	// Test basic find
	m.add("/path/to/handler", ep)
	f, _, _ := pm.find("GET", "/path/to/handler")
	if f != ep {
		t.Errorf(err)
	}

	// Test wc find
	m.add("/path/{to}/handler", ep)
	f, v, _ = pm.find("GET", "/path/wc/handler")
	if f != ep {
		t.Errorf(err)
	}
	found := false
	for _, p := range v {
		if p.key == "to" && p.value == "wc" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf(err)
	}

	// Test not found
	_, _, e := pm.find("GET", "/path/to/nf")
	if e != ErrNotFound {
		t.Errorf(e.Error())
	}

	// Test redirect
	_, _, e = pm.find("GET", "/path/to/handler/")
	if e != ErrRedirectSlash {
		t.Errorf(e.Error())
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
	f, _, _ := pm.find("GET", "/path/to/handler")
	f.serveHTTP(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	}

	// Test handler overwrite
	pm.Add("GET", "/path/to/handler", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			tVal = "B"
		},
	))
	f, _, _ = pm.find("GET", "/path/to/handler")
	f.serveHTTP(nil, nil)
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
	f, _, _ := pm.find("GET", "/path/to/handler")
	f.serveHTTP(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	}
}

func TestPathMuxerGroup(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed group"
	pm := New()

	// Test simple group
	g1 := pm.Group("GET", "/group1")

	// Test existing group
	g2 := pm.Group("GET", "/group1")
	if g2 != g1 {
		t.Errorf(err)
	}

	// Test group corruption of paths
	tVal := ""
	pm = New()
	pm.AddFunc("GET", "/path/to/handler", func(w http.ResponseWriter, r *http.Request) {
		tVal = "A"
	})
	g1 = pm.Group("GET", "/group1")
	f, _, _ := pm.find("GET", "/path/to/handler")
	f.serveHTTP(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	}

	// Test subsume endpoint
	tVal = ""
	g2 = pm.Group("GET", "/path/to")
	f, _, _ = pm.find("GET", "/path/to/handler")
	if group, ok := f.(*group); !ok {
		t.Errorf(err)
	} else if group.path != "/path/to" {
		t.Errorf(err)
	}
	r, _ := http.NewRequest("GET", "http://test.com/path/to/handler", nil)
	f.serveHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}

	// Test subsume group
	tVal = ""
	pm.Group("GET", "/path/")
	f, _, _ = pm.find("GET", "/path/to/handler")
	if group, ok := f.(*group); !ok {
		t.Errorf(err)
	} else if group.path != "/path" {
		// Notice trailing slash should be cut off
		t.Errorf(err)
	}
	f.serveHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}

	// Test super group
	tVal = ""
	pm = New()
	pm.AddFunc("GET", "/path/to/handler", func(w http.ResponseWriter, r *http.Request) {
		tVal = "A"
	})
	g1 = pm.Group("GET", "/path")
	pm.Group("GET", "/to")
	f, _, _ = pm.find("GET", "/path/to/handler")
	if group, ok := f.(*group); !ok {
		t.Errorf(err)
	} else if group.path != "/path" {
		t.Errorf(err)
	}
	f.serveHTTP(nil, r)
	if tVal != "A" {
		t.Errorf(err)
	}

}

func TestPathMuxerUse(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

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
	pm.chain.Run(nil, r)
	r, _ = http.NewRequest("GET", "http://test.com/path/to/2", nil)
	pm.chain.Run(nil, r)
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
	pm.chain.Run(nil, r)
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
	pm.AddFunc("GET", "/path/to/handler2", func(w http.ResponseWriter, r *http.Request) {
		tVal = "B"
	})
	pm.AddFunc("GET", "/path/{wc: ^[0-9]+$}/handler", func(w http.ResponseWriter, r *http.Request) {
		tVal = r.FormValue("wc")
	})
	pm.AddFunc("GET", "/path/{wc: ^[0-8]+$}/handler2", func(w http.ResponseWriter, r *http.Request) {
		tVal = r.FormValue("wc") + "2"
	})

	w := httptest.NewRecorder()

	// Test successful request
	r, _ := http.NewRequest("GET", "http://test.com/path/to/handler", nil)
	pm.ServeHTTP(w, r)
	if tVal != "A" {
		t.Errorf(err)
	}
	tVal = ""
	r, _ = http.NewRequest("GET", "http://test.com/path/to/handler2", nil)
	pm.ServeHTTP(w, r)
	if tVal != "B" {
		t.Errorf(err)
	}
	tVal = ""
	r, _ = http.NewRequest("GET", "http://test.com/path/1/handler", nil)
	pm.ServeHTTP(w, r)
	if tVal != "1" {
		t.Errorf(err)
	}
	tVal = ""
	r, _ = http.NewRequest("GET", "http://test.com/path/1/handler2", nil)
	pm.ServeHTTP(w, r)
	if tVal != "12" {
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
