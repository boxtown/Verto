// path_muxer_test
package mux

import (
	"bytes"
	"io"
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

	m := NewMuxNode()
	m.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tVal = "end"
	})

	// Test use one
	m.Use(PluginFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		tVal2 = "A"
		next(w, r)
	}))
	m.chain.run(nil, nil)

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
	m.chain.run(nil, nil)

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
	m = NewMuxNode()
	m.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	r, _ := http.NewRequest("GET", "http://test.com/", nil)
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

	m := NewMuxNode()
	m.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tVal = "end"
	})
	m.UseHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tVal2 = "A"
	}))
	m.chain.run(nil, nil)

	if tVal != "end" {
		t.Errorf(err)
	}
	if tVal2 != "A" {
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
	m := NewMuxNode()

	// Test basic find
	pm.matcher.Add("GET", "/path/to/handler", m)
	node, _, _ := pm.find("GET", "/path/to/handler")
	if node != m {
		t.Errorf(err)
	}

	// Test find with non-MuxNode
	pm.matcher.Add("GET", "/path/to/badnode", "A")
	_, _, e := pm.find("GET", "/path/to/badnode")
	if e != ErrNotFound {
		t.Errorf(err)
	}

	// Test not found
	_, _, e = pm.find("GET", "/path/to/nf")
	if e != ErrNotFound {
		t.Errorf(e.Error())
	}

	// Test not implemented
	_, _, e = pm.find("POST", "/path/to/handler")
	if e != ErrNotImplemented {
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
	n, _, _ := pm.find("GET", "/path/to/handler")
	n.handler.ServeHTTP(nil, nil)
	if tVal != "A" {
		t.Errorf(err)
	}

	// Test handler overwrite
	pm.Add("GET", "/path/to/handler", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			tVal = "B"
		},
	))
	n, _, _ = pm.find("GET", "/path/to/handler")
	n.handler.ServeHTTP(nil, nil)
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
	n, _, _ := pm.find("GET", "/path/to/handler")
	n.handler.ServeHTTP(nil, nil)
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

type BufResponseWriter struct {
	io.Writer

	status int
	header http.Header
}

func (brw *BufResponseWriter) Header() http.Header {
	return brw.header
}

func (brw *BufResponseWriter) Write(b []byte) (int, error) {
	return brw.Writer.Write(b)
}

func (brw *BufResponseWriter) WriteHeader(status int) {
	brw.status = status
}

func TestNotFoundHandler(t *testing.T) {
	err := "Failed not found handler."

	var buf bytes.Buffer
	brw := &BufResponseWriter{&buf, 0, http.Header{}}

	nfh := NotFoundHandler{}
	nfh.ServeHTTP(brw, nil)

	if buf.String() != "Not Found." {
		t.Errorf(err)
	}
	if brw.status != 404 {
		t.Errorf(err)
	}
}

func TestNotImplementedHandler(t *testing.T) {
	err := "Failed not implemented handler."

	var buf bytes.Buffer
	brw := &BufResponseWriter{&buf, 0, http.Header{}}

	nih := NotImplementedHandler{}
	nih.ServeHTTP(brw, nil)

	if buf.String() != "Not Implemented." {
		t.Errorf(err)
	}
	if brw.status != 501 {
		t.Errorf(err)
	}
}

func TestRedirectHandler(t *testing.T) {
	err := "Failed not redirect handler."

	var buf bytes.Buffer
	brw := &BufResponseWriter{&buf, 0, http.Header{}}

	r, _ := http.NewRequest("GET", "http://test.com", nil)

	rh := RedirectHandler{}
	rh.ServeHTTP(brw, r)

	if brw.Header().Get("Location") != "http://test.com" {
		t.Errorf(err)
	}
	if brw.status != 301 {
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

	var buf bytes.Buffer
	brw := &BufResponseWriter{&buf, 0, http.Header{}}

	// Test successful request
	r, _ := http.NewRequest("GET", "http://test.com/path/to/handler", nil)
	pm.ServeHTTP(brw, r)
	if tVal != "A" {
		t.Errorf(err)
	}

	// Test clean path
	brw = &BufResponseWriter{new(bytes.Buffer), 0, http.Header{}}
	r, _ = http.NewRequest("GET", "http://test.com/path/./to/../to/handler", nil)
	pm.ServeHTTP(brw, r)
	if brw.status != 301 {
		t.Errorf(err)
	}
	if brw.Header().Get("Location") != "http://test.com/path/to/handler" {
		t.Errorf(err)
	}

	// Test not found
	brw = &BufResponseWriter{new(bytes.Buffer), 0, http.Header{}}
	r, _ = http.NewRequest("GET", "http://test.com/nonexistent", nil)
	pm.ServeHTTP(brw, r)
	if brw.status != 404 {
		t.Errorf(err)
	}

	// Test not implemented
	brw = &BufResponseWriter{new(bytes.Buffer), 0, http.Header{}}
	r, _ = http.NewRequest("POST", "http://test.com/path/to/handler", nil)
	pm.ServeHTTP(brw, r)
	if brw.status != 501 {
		t.Error(err)
	}

	// Test redirect
	brw = &BufResponseWriter{new(bytes.Buffer), 0, http.Header{}}
	r, _ = http.NewRequest("GET", "http://test.com/path/to/handler/", nil)
	pm.Strict = false
	pm.ServeHTTP(brw, r)
	if brw.status != 301 {
		t.Errorf(err)
	}
	if brw.Header().Get("Location") != "http://test.com/path/to/handler" {
		t.Errorf(err)
	}
}
