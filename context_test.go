package verto

import (
	"net/http"
	"testing"
)

func TestContextGet(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed get."

	// Test improper initialization
	c := NewContext(nil, nil, nil, nil)
	v := c.Get("a")
	if v != "" {
		t.Errorf(err)
	}
	if c.ParseError() != ErrContextNotInitialized {
		t.Errorf(err)
	}

	// Test get
	r, _ := http.NewRequest("GET", "http://test.com?a=b", nil)
	c = NewContext(nil, r, nil, nil)
	v = c.Get("a")
	if v != "b" {
		t.Errorf(err)
	}
}

func TestContextGetMulti(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed get multi."

	// Test get
	r, _ := http.NewRequest("GET", "http://test.com?a=b&a=c", nil)
	c := NewContext(nil, r, nil, nil)
	v := c.GetMulti("a")
	if v[0] != "b" {
		t.Errorf(err)
	}
	if v[1] != "c" {
		t.Errorf(err)
	}
}

func TestContextGetBool(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed get bool."

	r, _ := http.NewRequest("GET", "http://test.com?a=true", nil)
	c := NewContext(nil, r, nil, nil)
	v, e := c.GetBool("a")
	if e != nil {
		t.Errorf(err)
	}
	if v != true {
		t.Errorf(err)
	}
}

func TestContextGetFloat64(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed get float64."

	r, _ := http.NewRequest("GET", "http://test.com?a=1.5", nil)
	c := NewContext(nil, r, nil, nil)
	v, e := c.GetFloat64("a")
	if e != nil {
		t.Errorf(err)
	}
	if v != 1.5 {
		t.Errorf(err)
	}
}

func TestContextGetInt64(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed get int64."

	r, _ := http.NewRequest("GET", "http://test.com?a=1", nil)
	c := NewContext(nil, r, nil, nil)
	v, e := c.GetInt64("a")
	if e != nil {
		t.Errorf(err)
	}
	if v != 1 {
		t.Errorf(err)
	}
}

func TestContextSet(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed set."

	// Test empty
	c := NewContext(nil, nil, nil, nil)
	c.Set("a", "b")
	if c.Get("a") != "" {
		t.Errorf(err)
	}
	if c.ParseError() != ErrContextNotInitialized {
		t.Errorf(err)
	}

	// Test set
	r, _ := http.NewRequest("GET", "http://test.com", nil)
	c = NewContext(nil, r, nil, nil)
	c.Set("c", "d")
	v := c.Get("c")
	if v != "d" {
		t.Errorf(err)
	}
}

func TestContextSetMulti(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed set multi."

	// Test set multi
	m := make([]string, 2)
	m[0] = "b"
	m[1] = "c"
	r, _ := http.NewRequest("GET", "http://test.com", nil)
	c := NewContext(nil, r, nil, nil)
	c.SetMulti("a", m)
	v := c.GetMulti("a")
	if v[0] != "b" {
		t.Errorf(err)
	}
	if v[1] != "c" {
		t.Errorf(err)
	}
}

func TestContextSetBool(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed set bool."

	r, _ := http.NewRequest("GET", "http://test.com", nil)
	c := NewContext(nil, r, nil, nil)
	c.SetBool("a", true)
	v, e := c.GetBool("a")
	if e != nil {
		t.Errorf(err)
	}
	if v != true {
		t.Errorf(err)
	}
}

func TestContextSetFloat64(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed set float64."

	r, _ := http.NewRequest("GET", "http://test.com", nil)
	c := NewContext(nil, r, nil, nil)
	c.SetFloat64("a", 1.5, 'G', -1)
	v, e := c.GetFloat64("a")
	if e != nil {
		t.Errorf(err)
	}
	if v != 1.5 {
		t.Errorf(err)
	}
}

func TestContextSetInt64(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed set int64."

	r, _ := http.NewRequest("GET", "http://test.com", nil)
	c := NewContext(nil, r, nil, nil)
	c.SetInt64("a", 1)
	v, e := c.GetInt64("a")
	if e != nil {
		t.Errorf(err)
	}
	if v != 1 {
		t.Errorf(err)
	}
}
