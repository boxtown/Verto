package verto

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestLoggerAddSubscriber(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed add subscriber."

	l := NewLogger()
	defer l.Close()

	for i := 0; i < 10; i++ {
		c := l.AddSubscriber(strconv.FormatInt(int64(i), 10))
		go func() {
			s := <-c
			test := getMessage(s)
			if test != "test" {
				t.Errorf(err)
			}
		}()
	}

	l.Printf("test")
}

func TestLoggerAddFile(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed add file."

	l := NewLogger()
	defer l.Close()

	r, w, e := os.Pipe()
	if e != nil {
		t.Errorf(e.Error())
	}

	l.AddFile(w)
	l.Printf("test")

	b := make([]byte, 256)
	n, e := r.Read(b)
	if e != nil {
		t.Errorf(e.Error())
	}

	test := getMessage(string(b[:n]))
	if test != "test" {
		t.Error(err)
	}
}

func TestLoggerAddFilePath(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	l := NewLogger()
	defer l.Close()

	e := l.AddFilePath("testFile")
	if e != nil {
		t.Skipf(e.Error())
	}

	l.Printf("test")

	f, e := os.Open("testFile")
	if e != nil {
		t.Errorf(e.Error())
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var s string
	for scanner.Scan() {
		s = scanner.Text()
	}

	test := getMessage(s)
	if test != "test" {
		t.Errorf(test)
	}
}

func getMessage(logMsg string) string {
	sp := strings.Split(logMsg, ":")
	return strings.TrimSpace(sp[len(sp)-1])
}

func getPrefix(logMsg string) string {
	msg := getMessage(logMsg)
	sp := strings.Split(msg, " ")
	return sp[0]
}
