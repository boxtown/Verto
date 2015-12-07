package compression

import (
	"compress/flate"
	"compress/gzip"
	"io"
)

// writerRef is a long-lived pooled reference to an io.WriteCloser
type writerRef struct {
	w        io.WriteCloser
	disposal chan<- io.WriteCloser
}

// dispose disposes of the reference by either returning the
// writer to the pool or disposing of the writer if
// the pool is full
func (ref *writerRef) dispose() {
	switch w := ref.w.(type) {
	case *flate.Writer:
		w.Flush()
	case *gzip.Writer:
		w.Flush()
	}

	select {
	case ref.disposal <- ref.w:
	default:
		ref.w.Close()
	}
}

// compressType represents a compression type
type compressType int64

const (
	// ctFlate represents a flate compression type
	ctFlate compressType = 0

	// ctGzip represents a gzip compression type
	ctGzip compressType = 1
)

// writerPool is a concurrency-safe pool of compression writers
type writerPool struct {
	flatePool chan io.WriteCloser
	gzipPool  chan io.WriteCloser
}

// newWriterPool returns a compressWriterPool reference
// with maxWriters as the maximum number of poolable writers per
// compressionType
func newWriterPool(maxWriters int) *writerPool {
	return &writerPool{
		flatePool: make(chan io.WriteCloser, maxWriters),
		gzipPool:  make(chan io.WriteCloser, maxWriters),
	}
}

// get attempts to retrieve a writer of type ct from the pool and wrap it
// around inner or, if no writers are available creates a new writer of type
// ct around inner
func (pool *writerPool) get(inner io.Writer, ct compressType) *writerRef {
	var w io.WriteCloser
	var disposal chan<- io.WriteCloser

	switch ct {
	case ctFlate:
		select {
		case w = <-pool.flatePool:
			w.(*flate.Writer).Reset(inner)
			disposal = pool.flatePool
		default:
			w, _ = flate.NewWriter(inner, flate.DefaultCompression)
			disposal = pool.flatePool
		}
	case ctGzip:
		select {
		case w = <-pool.gzipPool:
			w.(*gzip.Writer).Reset(inner)
			disposal = pool.gzipPool
		default:
			w = gzip.NewWriter(inner)
			disposal = pool.gzipPool
		}
	}
	return &writerRef{w, disposal}
}

// global pool of compression writers
var pool = newWriterPool(1000)
