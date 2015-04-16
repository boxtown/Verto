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
	Info(v ...interface{}) error
	Debug(v ...interface{}) error
	Warn(v ...interface{}) error
	Error(v ...interface{}) error

	Infof(format string, v ...interface{}) error
	Debugf(format string, v ...interface{}) error
	Warnf(format string, v ...interface{}) error
	Errorf(format string, v ...interface{}) error

	Print(v ...interface{}) error
	Printf(format string, v ...interface{}) error

	AddSubscriber(key string) <-chan string
	AddFile(f *os.File) error
	AddFilePath(path string) error

	Dropped(key string) []string

	Close() error
}

// DefaultLogger is the Verto default implementation of verto.Logger.
type DefaultLogger struct {
	subscribers map[string]chan string
	dropped     map[string][]string
	files       []*os.File
}

// NewLogger returns a pointer to a newly initialized VertoLogger instance.
func NewLogger() *DefaultLogger {
	return &DefaultLogger{
		subscribers: make(map[string]chan string),
		dropped:     make(map[string][]string),
		files:       make([]*os.File, 0),
	}
}

// AddSubscriber adds a channel between the logger and subscriber to
// VertoLogger. Any messages written by VertoLogger will be piped out to
// the returned channel. NOTE: If a previous subscriber with the same key exists,
// it will be OVERWRITTEN.
func (dl *DefaultLogger) AddSubscriber(key string) <-chan string {
	dl.subscribers[key] = make(chan string)
	dl.dropped[key] = make([]string, 0)
	return dl.subscribers[key]
}

// AddFile registers an open file for logging. Returns
// an error if a bad file is passed in.
func (dl *DefaultLogger) AddFile(f *os.File) error {
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

// Close attempts to close all opened files attached to VertoLogger.
// Errors are recorded and combined into one single error so that an
// error doesn't prevent the closing of other files.
func (dl *DefaultLogger) Close() error {
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

	return errors.New(buf.String())
}

// Info prints an info level message to all subscribers and open
// log files. Info returns an error if there was an error printing.
func (dl *DefaultLogger) Info(v ...interface{}) error {
	prefix := "[INFO]"
	return dl.lprint(prefix, v...)
}

// Debug prints a debug level message to all subscribers and open
// log files. Debug returns an error if there was an error printing.
func (dl *DefaultLogger) Debug(v ...interface{}) error {
	prefix := "[DEBUG]"
	return dl.lprint(prefix, v...)
}

// Warn prints a warn level message to all subscribers and open
// log files. Warn returns an error if there was an error printing.
func (rl *DefaultLogger) Warn(v ...interface{}) error {
	prefix := "[WARN]"
	return rl.lprint(prefix, v...)
}

// Error prints an error level message to all subscribers and open
// log files. Error returns an error if there was an error printing.
func (dl *DefaultLogger) Error(v ...interface{}) error {
	prefix := "[ERROR]"
	return dl.lprint(prefix, v...)
}

// Infof prints a formatted info level message to all subscribers and open
// log files. Info returns an error if there was an error printing.
func (dl *DefaultLogger) Infof(format string, v ...interface{}) error {
	prefix := "[INFO]"
	return dl.lprintf(prefix, format, v...)
}

// Debugf prints a formatted debug level message to all subscribers and open
// log files. Debug returns an error if there was an error printing.
func (dl *DefaultLogger) Debugf(format string, v ...interface{}) error {
	prefix := "[DEBUG]"
	return dl.lprintf(prefix, format, v...)
}

// Warnf prints a formatted warn level message to all subscribers and open
// log files. Warn returns an error if there was an error printing.
func (dl *DefaultLogger) Warnf(format string, v ...interface{}) error {
	prefix := "[WARN]"
	return dl.lprintf(prefix, format, v...)
}

// Errorf prints a formmated error level message to all subscribers and open
// log files. Error returns an error if there was an error printing.
func (dl *DefaultLogger) Errorf(format string, v ...interface{}) error {
	prefix := "[ERROR]"
	return dl.lprintf(prefix, format, v...)
}

// Print prints a message to all subscribers and open
// log files. Print returns an error if there was an error printing.
func (dl *DefaultLogger) Print(v ...interface{}) error {
	return dl.lprint("", v...)
}

// Printf prints a formatted message to all subscribers and open
// log files. Printf returns an error if there was an error printing.
func (dl *DefaultLogger) Printf(format string, v ...interface{}) error {
	return dl.lprintf("", format, v...)
}

// Prints a message to all subscribers and open log files.
// Returns an error if there was an error printing.
func (dl *DefaultLogger) lprint(prefix string, v ...interface{}) error {
	var buf bytes.Buffer
	dl.appendPrefix(prefix, &buf)

	buf.WriteString(fmt.Sprint(v))
	buf.WriteString("\n")

	msg := buf.String()

	dl.pushToSubs(msg)
	return dl.writeToFiles(msg)
}

// Prints a formatted message.
func (dl *DefaultLogger) lprintf(prefix, format string, v ...interface{}) error {
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
	return dl.writeToFiles(msg)
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
