package compression

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"github.com/boxtown/verto"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompressionPlugin(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed compression."

	plugin := New()

	endpoint := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test"))
	})

	// Test no compression
	r, _ := http.NewRequest("GET", "http://test.com", nil)
	w := httptest.NewRecorder()
	c := &verto.Context{Request: r, Response: w}

	plugin.Handle(c, endpoint)
	if w.Body.String() != "test" {
		t.Errorf(err)
	}
	if w.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf(err)
	}

	// Test gzip compression
	r, _ = http.NewRequest("GET", "http://test.com", nil)
	r.Header.Add("Accept-Encoding", "gzip")
	w = httptest.NewRecorder()
	c = &verto.Context{Request: r, Response: w}
	plugin.Handle(c, endpoint)

	reader := bytes.NewReader(w.Body.Bytes())
	gr, _ := gzip.NewReader(reader)
	defer gr.Close()

	b2 := make([]byte, len([]byte("test")))
	gr.Read(b2)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf(err)
	}
	if string(b2) != "test" {
		t.Errorf(err)
	}

	// Test deflate compression
	r, _ = http.NewRequest("GET", "http://test.com", nil)
	r.Header.Add("Accept-Encoding", "deflate")
	w = httptest.NewRecorder()
	c = &verto.Context{Request: r, Response: w}
	plugin.Handle(c, endpoint)

	reader = bytes.NewReader(w.Body.Bytes())
	fr := flate.NewReader(reader)
	defer fr.Close()

	b2 = make([]byte, len([]byte("test")))
	fr.Read(b2)

	if w.Header().Get("Content-Encoding") != "deflate" {
		t.Errorf(err)
	}
	if string(b2) != "test" {
		t.Errorf(err)
	}
}
