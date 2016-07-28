package verto

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestLoggerPrinting(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	l := NewLogger()
	defer l.Close()

	msg := "test"
	for i := 0; i < 10; i++ {
		key := strconv.FormatInt(int64(i), 10)
		c := make(chan string)
		l.subscribers[key] = c
		go func(sub <-chan string) {
			for {
				_, ok := <-sub
				if !ok {
					return
				}
			}
		}(c)
	}

	for i := 0; i < 10; i++ {
		r, w, e := os.Pipe()
		if e != nil {
			t.Errorf(e.Error())
		}
		defer r.Close()

		l.files = append(l.files, w)
	}

	l.Info(msg)
	l.Debug(msg)
	l.Warn(msg)
	l.Error(msg)
	l.Print(msg)
	l.Infof("%s", msg)
	l.Debugf("%s", msg)
	l.Warnf("%s", msg)
	l.Errorf("%s", msg)
	l.Printf("%s", msg)
}

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

	dropMap := make(map[string]chan bool)

	for i := 0; i < 10; i++ {

		k := strconv.FormatInt(int64(i), 10)
		c := l.AddSubscriber(k)
		dropMap[k] = make(chan bool)

		go func() {
			s := <-c
			test := getMessage(s)
			if test != "test" {
				dropMap[k] <- true
			} else {
				dropMap[k] <- false
			}
		}()
	}

	l.Printf("test")

	for k, v := range dropMap {
		if <-v {
			dropped := l.Dropped(k)
			if getMessage(dropped[0]) != "test" {
				t.Errorf(err)
			}
		}
	}
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
	defer r.Close()

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
