package verto

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"
)

// Logger is the interface for Verto Logging.
type Logger interface {
	Info(format string, v ...interface{}) error
	Debug(format string, v ...interface{}) error
	Warn(format string, v ...interface{}) error
	Error(format string, v ...interface{}) error
	Printf(format string, v ...interface{}) error

	AddSubscriber(key string) <-chan string
	AddFile(f *os.File) error
	AddFilePath(path string) error

	Close() error
}

// VertoLogger is the Verto default implementation of verto.Logger.
type VertoLogger struct {
	subscribers map[string]chan string
	files       []*os.File
}

// NewLogger returns a pointer to a newly initialized VertoLogger instance.
func NewLogger() *VertoLogger {
	return &VertoLogger{
		subscribers: make(map[string]chan string),
		files:       make([]*os.File, 0),
	}
}

// Add subscriber adds a channel between the logger and subscriber to
// VertoLogger. Any messages written by VertoLogger will be piped out to
// the returned channel. NOTE: If a previous subscriber with the same key exists,
// it will be OVERWRITTEN.
func (vl *VertoLogger) AddSubscriber(key string) <-chan string {
	vl.subscribers[key] = make(chan string)
	return vl.subscribers[key]
}

func (vl *VertoLogger) AddFile(f *os.File) error {
	if f == nil {
		return errors.New("logger.AddFile: bad file as argument")
	}

	vl.files = append(vl.files, f)
	return nil
}

// AddFile attempts to open the file at path as append-only
// and will begin writing messages to the file or return an error
// if an error occured opening up the file.
func (vl *VertoLogger) AddFilePath(path string) error {
	f, err := os.OpenFile(path, os.O_APPEND, os.ModeAppend)
	if err != nil {
		return err
	}

	vl.files = append(vl.files, f)
	return nil
}

// Close attempts to close all opened files attached to VertoLogger.
// Errors are recorded and combined into one single error so that an
// error doesn't prevent the closing of other files.
func (vl *VertoLogger) Close() error {
	var buf bytes.Buffer
	for _, v := range vl.files {
		err := v.Close()
		if err != nil {
			buf.WriteString(err.Error())
			buf.WriteString("\n")
		}
	}

	return errors.New(buf.String())
}

// Info prints an info level message to all subscribers and open
// log files. Info returns an error if there was an error printing.
func (vl *VertoLogger) Info(format string, v ...interface{}) error {
	prefix := "[INFO]"
	return vl.printf(prefix, format, v...)
}

// Debug prints a debug level message to all subscribers and open
// log files. Debug returns an error if there was an error printing.
func (vl *VertoLogger) Debug(format string, v ...interface{}) error {
	prefix := "[DEBUG]"
	return vl.printf(prefix, format, v...)
}

// Warm prints a warn level message to all subscribers and open
// log files. Warn returns an error if there was an error printing.
func (vl *VertoLogger) Warn(format string, v ...interface{}) error {
	prefix := "[WARN]"
	return vl.printf(prefix, format, v...)
}

// Error prints an error level message to all subscribers and open
// log files. Error returns an error if there was an error printing.
func (vl *VertoLogger) Error(format string, v ...interface{}) error {
	prefix := "[ERROR]"
	return vl.printf(prefix, format, v...)
}

// Printf prints a message to all subscribers and open
// log files. Printf returns an error if there was an error printing.
func (vl *VertoLogger) Printf(format string, v ...interface{}) error {
	return vl.printf("", format, v...)
}

func (vl *VertoLogger) printf(prefix, format string, v ...interface{}) error {
	var errBuf bytes.Buffer
	var buf bytes.Buffer
	buf.WriteString(time.Now().String())
	buf.WriteString(": ")
	buf.WriteString(prefix)
	buf.WriteString(" ")
	buf.WriteString(fmt.Sprintf(format, v))

	msg := buf.String()

	for _, s := range vl.subscribers {
		s <- msg
	}

	for _, f := range vl.files {
		_, err := f.WriteString(msg)
		if err != nil {
			errBuf.WriteString(err.Error())
			errBuf.WriteString("\n")
		}
	}

	if errBuf.Len() > 0 {
		return errors.New(errBuf.String())
	}

	return nil
}
