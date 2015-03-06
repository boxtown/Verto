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
	f, err := os.OpenFile(path, os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	_, err = f.WriteString("\n")
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

	for _, v := range vl.subscribers {
		close(v)
	}

	return errors.New(buf.String())
}

// Info prints an info level message to all subscribers and open
// log files. Info returns an error if there was an error printing.
func (vl *VertoLogger) Info(v ...interface{}) error {
	prefix := "[INFO]"
	return vl.print(prefix, v...)
}

// Debug prints a debug level message to all subscribers and open
// log files. Debug returns an error if there was an error printing.
func (vl *VertoLogger) Debug(v ...interface{}) error {
	prefix := "[DEBUG]"
	return vl.print(prefix, v...)
}

// Warn prints a warn level message to all subscribers and open
// log files. Warn returns an error if there was an error printing.
func (vl *VertoLogger) Warn(v ...interface{}) error {
	prefix := "[WARN]"
	return vl.print(prefix, v...)
}

// Error prints an error level message to all subscribers and open
// log files. Error returns an error if there was an error printing.
func (vl *VertoLogger) Error(v ...interface{}) error {
	prefix := "[ERROR]"
	return vl.print(prefix, v...)
}

// Infof prints a formatted info level message to all subscribers and open
// log files. Info returns an error if there was an error printing.
func (vl *VertoLogger) Infof(format string, v ...interface{}) error {
	prefix := "[INFO]"
	return vl.printf(prefix, format, v...)
}

// Debugf prints a formatted debug level message to all subscribers and open
// log files. Debug returns an error if there was an error printing.
func (vl *VertoLogger) Debugf(format string, v ...interface{}) error {
	prefix := "[DEBUG]"
	return vl.printf(prefix, format, v...)
}

// Warnf prints a formatted warn level message to all subscribers and open
// log files. Warn returns an error if there was an error printing.
func (vl *VertoLogger) Warnf(format string, v ...interface{}) error {
	prefix := "[WARN]"
	return vl.printf(prefix, format, v...)
}

// Errorf prints a formmated error level message to all subscribers and open
// log files. Error returns an error if there was an error printing.
func (vl *VertoLogger) Errorf(format string, v ...interface{}) error {
	prefix := "[ERROR]"
	return vl.printf(prefix, format, v...)
}

// Print prints a message to all subscribers and open
// log files. Print returns an error if there was an error printing.
func (vl *VertoLogger) Print(v ...interface{}) error {
	return vl.print("", v...)
}

// Printf prints a formatted message to all subscribers and open
// log files. Printf returns an error if there was an error printing.
func (vl *VertoLogger) Printf(format string, v ...interface{}) error {
	return vl.printf("", format, v...)
}

// Prints a message to all subscribers and open log files.
// Returns an error if there was an error printing.
func (vl *VertoLogger) print(prefix string, v ...interface{}) error {
	var buf bytes.Buffer
	vl.appendPrefix(prefix, buf)

	buf.WriteString(fmt.Sprint(v))

	msg := buf.String()

	vl.pushToSubs(msg)
	return vl.writeToFiles(msg)
}

// Prints a formatted message.
func (vl *VertoLogger) printf(prefix, format string, v ...interface{}) error {
	var buf bytes.Buffer
	vl.appendPrefix(prefix, buf)

	if len(v) > 0 {
		buf.WriteString(fmt.Sprintf(format, v))
	} else {
		buf.WriteString(fmt.Sprint(format))
	}

	msg := buf.String()

	vl.pushToSubs(msg)
	return vl.writeToFiles(msg)
}

// Appends a prefix consisting of the current time and the passed in prefix
// to a byte Buffer. Assumes the buffer is valid (not nil).
func (vl *VertoLogger) appendPrefix(prefix string, buf bytes.Buffer) {
	buf.WriteString(time.Now().String())
	buf.WriteString(": ")
	buf.WriteString(prefix)
	buf.WriteString(" ")
}

// Pushes a string message to all subscribers.
func (vl *VertoLogger) pushToSubs(msg string) {
	for _, s := range vl.subscribers {
		s <- msg
	}
}

// Writes a string message to all open log files.
func (vl *VertoLogger) writeToFiles(msg string) error {
	var errBuf bytes.Buffer

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
