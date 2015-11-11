package verto

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// Logger is the interface for Verto Logging.
type Logger interface {
	Info(v ...interface{})
	Debug(v ...interface{})
	Warn(v ...interface{})
	Error(v ...interface{})
	Fatal(v ...interface{})
	Panic(v ...interface{})

	Infof(format string, v ...interface{})
	Debugf(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Fatalf(format string, v ...interface{})
	Panicf(format string, v ...interface{})

	Print(v ...interface{})
	Printf(format string, v ...interface{})

	Close() error
}

// NilLogger is a logger that implements the logging
// interface that discards all log messages
type NilLogger struct{}

func (nl *NilLogger) Info(v ...interface{})                  {}
func (nl *NilLogger) Debug(v ...interface{})                 {}
func (nl *NilLogger) Warn(v ...interface{})                  {}
func (nl *NilLogger) Error(v ...interface{})                 {}
func (nl *NilLogger) Fatal(v ...interface{})                 {}
func (nl *NilLogger) Panic(v ...interface{})                 {}
func (nl *NilLogger) Infof(format string, v ...interface{})  {}
func (nl *NilLogger) Debugf(format string, v ...interface{}) {}
func (nl *NilLogger) Warnf(format string, v ...interface{})  {}
func (nl *NilLogger) Errorf(format string, v ...interface{}) {}
func (nl *NilLogger) Fatalf(format string, v ...interface{}) {}
func (nl *NilLogger) Panicf(format string, v ...interface{}) {}
func (nl *NilLogger) Print(v ...interface{})                 {}
func (nl *NilLogger) Printf(format string, v ...interface{}) {}
func (nl *NilLogger) Close() error                           { return nil }

// DefaultLogger is the Verto default implementation of verto.Logger.
type DefaultLogger struct {
	subscribers map[string]chan string
	dropped     map[string][]string
	errors      []string
	files       []*os.File
	closed      bool
	mut         *sync.Mutex
}

// NewLogger returns a pointer to a newly initialized VertoLogger instance.
func NewLogger() *DefaultLogger {
	return &DefaultLogger{
		subscribers: make(map[string]chan string),
		dropped:     make(map[string][]string),
		errors:      make([]string, 0),
		files:       make([]*os.File, 0),
		closed:      false,
		mut:         &sync.Mutex{},
	}
}

// AddSubscriber adds a channel between the logger and subscriber to
// VertoLogger. Any messages written by VertoLogger will be piped out to
// the returned channel. NOTE: If a previous subscriber with the same key exists,
// it will be OVERWRITTEN.
func (dl *DefaultLogger) AddSubscriber(key string) <-chan string {
	dl.mut.Lock()
	defer dl.mut.Unlock()

	dl.subscribers[key] = make(chan string)
	dl.dropped[key] = make([]string, 0)
	return dl.subscribers[key]
}

// AddFile registers an open file for logging. Returns
// an error if a bad file is passed in.
func (dl *DefaultLogger) AddFile(f *os.File) error {
	dl.mut.Lock()
	defer dl.mut.Unlock()

	if f == nil {
		return errors.New("logger.AddFile: bad file as argument")
	}

	dl.files = append(dl.files, f)
	return nil
}

// AddFilePath attempts to open the file at path as append-only
// and will begin writing messages to the file or return an error
// if an error occured opening up the file.
func (dl *DefaultLogger) AddFilePath(path string) error {
	dl.mut.Lock()
	defer dl.mut.Unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	_, err = f.WriteString("\n")
	if err != nil {
		return err
	}

	dl.files = append(dl.files, f)
	return nil
}

// Dropped returns a slice of strings representing
// any dropped log messages due to timeout sends to
// the subscriber described by key.
func (dl *DefaultLogger) Dropped(key string) []string {
	return dl.dropped[key]
}

// Errors returns a slice of all errors that occured
// while writing to files
func (dl *DefaultLogger) Errors() []string {
	return dl.errors
}

// Close attempts to close all opened files attached to VertoLogger.
// Errors are recorded and combined into one single error so that an
// error doesn't prevent the closing of other files.
func (dl *DefaultLogger) Close() error {
	dl.mut.Lock()
	if dl.closed {
		dl.mut.Unlock()
		return nil
	}
	dl.closed = true
	dl.mut.Unlock()

	var buf bytes.Buffer
	for _, v := range dl.files {
		err := v.Close()
		if err != nil {
			buf.WriteString(err.Error())
			buf.WriteString("\n")
		}
	}

	for _, v := range dl.subscribers {
		close(v)
	}

	if buf.Len() > 0 {
		return errors.New(buf.String())
	}
	return nil
}

// Info prints an info level message to all subscribers and open
// log files.
func (dl *DefaultLogger) Info(v ...interface{}) {
	prefix := "[INFO]"
	dl.lprint(prefix, v...)
}

// Debug prints a debug level message to all subscribers and open
// log files.
func (dl *DefaultLogger) Debug(v ...interface{}) {
	prefix := "[DEBUG]"
	dl.lprint(prefix, v...)
}

// Warn prints a warn level message to all subscribers and open
// log files.
func (dl *DefaultLogger) Warn(v ...interface{}) {
	prefix := "[WARN]"
	dl.lprint(prefix, v...)
}

// Error prints an error level message to all subscribers and open
// log files.
func (dl *DefaultLogger) Error(v ...interface{}) {
	prefix := "[ERROR]"
	dl.lprint(prefix, v...)
}

// Fatal prints a fatal level message to all subscribers and open log files
// and then calls os.Exit
func (dl *DefaultLogger) Fatal(v ...interface{}) {
	prefix := "[FATAL]"
	dl.lprint(prefix, v...)
	os.Exit(1)
}

// Panic prints a panic level message to all subscribers and open log files
// and then panics
func (dl *DefaultLogger) Panic(v ...interface{}) {
	prefix := "[PANIC]"
	dl.lprint(prefix, v...)
	panic(fmt.Sprint(v...))
}

// Infof prints a formatted info level message to all subscribers and open
// log files.
func (dl *DefaultLogger) Infof(format string, v ...interface{}) {
	prefix := "[INFO]"
	dl.lprintf(prefix, format, v...)
}

// Debugf prints a formatted debug level message to all subscribers and open
// log files.
func (dl *DefaultLogger) Debugf(format string, v ...interface{}) {
	prefix := "[DEBUG]"
	dl.lprintf(prefix, format, v...)
}

// Warnf prints a formatted warn level message to all subscribers and open
// log files.
func (dl *DefaultLogger) Warnf(format string, v ...interface{}) {
	prefix := "[WARN]"
	dl.lprintf(prefix, format, v...)
}

// Errorf prints a formatted error level message to all subscribers and open
// log files.
func (dl *DefaultLogger) Errorf(format string, v ...interface{}) {
	prefix := "[ERROR]"
	dl.lprintf(prefix, format, v...)
}

// Fatalf prints a formatted fatal level message to all subscribers and open log files
// and then calls os.Exit
func (dl *DefaultLogger) Fatalf(format string, v ...interface{}) {
	prefix := "[FATAL]"
	dl.lprintf(prefix, format, v...)
	os.Exit(1)
}

// Panicf prints a formatted panic level message to all subscribers and open log files
// and then panics
func (dl *DefaultLogger) Panicf(format string, v ...interface{}) {
	prefix := "[PANIC]"
	dl.lprintf(prefix, format, v...)
	panic(fmt.Sprintf(format, v...))
}

// Print prints a message to all subscribers and open
// log files.
func (dl *DefaultLogger) Print(v ...interface{}) {
	dl.lprint("", v...)
}

// Printf prints a formatted message to all subscribers and open
// log files.
func (dl *DefaultLogger) Printf(format string, v ...interface{}) {
	dl.lprintf("", format, v...)
}

// Prints a message to all subscribers and open log files. Keeps
// track of errors writing to files
func (dl *DefaultLogger) lprint(prefix string, v ...interface{}) {
	var buf bytes.Buffer
	dl.appendPrefix(prefix, &buf)

	buf.WriteString(fmt.Sprint(v))
	buf.WriteString("\n")

	msg := buf.String()

	dl.pushToSubs(msg)
	dl.writeToFiles(msg)
}

// Prints a formatted message. Keeps track of errors writing to files
func (dl *DefaultLogger) lprintf(prefix, format string, v ...interface{}) {
	var buf bytes.Buffer
	dl.appendPrefix(prefix, &buf)

	if len(v) > 0 {
		buf.WriteString(fmt.Sprintf(format, v))
	} else {
		buf.WriteString(fmt.Sprint(format))
	}
	buf.WriteString("\n")

	msg := buf.String()

	dl.pushToSubs(msg)
	dl.writeToFiles(msg)
}

// Appends a prefix consisting of the current time and the passed in prefix
// to a byte Buffer. Assumes the buffer is valid (not nil).
func (dl *DefaultLogger) appendPrefix(prefix string, buf *bytes.Buffer) {
	buf.WriteString(time.Now().String())
	buf.WriteString(": ")
	buf.WriteString(prefix)
	buf.WriteString(" ")
}

// Pushes a string message to all subscribers.
func (dl *DefaultLogger) pushToSubs(msg string) {
	for k, s := range dl.subscribers {
		select {
		case s <- msg:
			break
		case <-time.After(500 * time.Millisecond):
			dl.dropped[k] = append(dl.dropped[k], msg)
		}
	}
}

// Writes a string message to all open log files.
func (dl *DefaultLogger) writeToFiles(msg string) error {
	var errBuf bytes.Buffer

	for _, f := range dl.files {
		dl.mut.Lock()
		_, err := f.WriteString(msg)
		dl.mut.Unlock()

		if err != nil {
			errBuf.WriteString(err.Error())
			errBuf.WriteString("\n")
		}
	}

	if errBuf.Len() > 0 {
		errMsg := errBuf.String()
		dl.errors = append(dl.errors, errMsg)
		return errors.New(errMsg)
	}

	return nil
}
