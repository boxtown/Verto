package verto

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type compressionWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w compressionWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// CompressionPlugin returns a VertoPluginFunc that handles
// gzip/deflate encoding.
func CompressionPlugin() VertoPluginFunc {
	return VertoPluginFunc(compressionFunc)
}

func compressionFunc(c *Context, next http.HandlerFunc) {
	r := c.Request
	w := c.Response

	w.Header().Add("Vary", "Accept-Encoding")

	enc := strings.Split(r.Header.Get("Accept-Encoding"), ",")
	for _, v := range enc {
		v = strings.TrimSpace(v)
		if v == "gzip" {
			w.Header().Add("Content-Encoding", "gzip")

			gw := gzip.NewWriter(w)
			defer gw.Close()

			w = &compressionWriter{
				Writer:         gw,
				ResponseWriter: w,
			}
			next(w, r)
			return
		}
		if v == "deflate" {
			w.Header().Add("Content-Encoding", "deflate")

			fw, _ := flate.NewWriter(w, flate.DefaultCompression)
			defer fw.Close()

			w = &compressionWriter{
				Writer:         fw,
				ResponseWriter: w,
			}
			next(w, r)
			return
		}
	}

	next(w, r)
}
