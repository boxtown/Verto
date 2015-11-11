package plugins

import (
	"github.com/boxtown/verto"
	"io"
	"net/http"
	"strings"
)

// Compression is a plugin that replaces the default
// ResponseWriter with a compression writer that compresses
// everything written to the response. Currently supports
// gzip and deflate
type Compression struct {
	// Core is the core functionality for plugins
	Core
}

// NewCompression returns a newly initialized Compression plugin
func NewCompression() *Compression {
	return &Compression{Core: Core{id: "plugins.Compression"}}
}

// Handle is called on per web request to supply a compression writer to the
// other plugins and request handler. Currently only gzip and deflate are supported.
// The compression type used is the first supported compression type encountered
// in the 'Accept-Encoding' header of incoming requests
func (plugin *Compression) Handle(c *verto.Context, next http.HandlerFunc) {
	plugin.Core.Handle(
		func(c *verto.Context, next http.HandlerFunc) {
			r := c.Request
			w := c.Response

			w.Header().Add("Vary", "Accept-Encoding")

			enc := strings.Split(r.Header.Get("Accept-Encoding"), ",")
			for _, v := range enc {
				v = strings.ToLower(strings.TrimSpace(v))
				if v == "gzip" {
					w.Header().Add("Content-Encoding", "gzip")

					ref := pool.get(w, ctGzip)
					defer ref.dispose()

					w = &compressionWriter{
						Writer:         ref.w,
						ResponseWriter: w,
					}
					next(w, r)
					return
				}
				if v == "deflate" {
					w.Header().Add("Content-Encoding", "deflate")

					ref := pool.get(w, ctFlate)
					defer ref.dispose()

					w = &compressionWriter{
						Writer:         ref.w,
						ResponseWriter: w,
					}
					next(w, r)
					return
				}
			}
			next(w, r)
		}, c, next)
}

// compressionWriter implements io.Writer as well as http.ResponseWriter.
// It is assumed that the io.Writer is a compression writer that wraps
// the http.ResponseWriter
type compressionWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w compressionWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w compressionWriter) Write(b []byte) (int, error) {
	if len(w.Header().Get("Content-Type")) == 0 {
		w.Header().Set("Content-Type", http.DetectContentType(b))
	}
	return w.Writer.Write(b)
}

func (w compressionWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
}
