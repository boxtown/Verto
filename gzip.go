package verto

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type gzipWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func GzipPlugin(c *Context, next http.HandlerFunc) {
	r := c.Request
	w := c.Response

	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		next(w, r)
		return
	}

	w.Header().Set("Content-Encoding", "gzip")
	gz := gzip.NewWriter(w)
	defer gz.Close()
	next(gzipWriter{Writer: gz, ResponseWriter: w}, r)
}
