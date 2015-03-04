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
	c := &Context{}
	_, e := c.Get("a")
	if e != ErrContextNotInitialized {
		t.Errorf(err)
	}

	// Test get
	r, _ := http.NewRequest("GET", "http://test.com?a=b", nil)
	c = &Context{Request: r}
	v, _ := c.Get("a")
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

	// Test improper initialization
	c := &Context{}
	_, e := c.Get("a")
	if e != ErrContextNotInitialized {
		t.Errorf(err)
	}

	// Test get
	r, _ := http.NewRequest("GET", "http://test.com?a=b&a=c", nil)
	c = &Context{Request: r}
	v, _ := c.GetMulti("a")
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
	c := &Context{Request: r}
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
	c := &Context{Request: r}
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
	c := &Context{Request: r}
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

	// Test improper initialization
	c := &Context{}
	e := c.Set("a", "b")
	if e != ErrContextNotInitialized {
		t.Errorf(err)
	}

	// Test set
	r, _ := http.NewRequest("GET", "http://test.com", nil)
	c = &Context{Request: r}
	e = c.Set("a", "b")
	if e != nil {
		t.Errorf(err)
	}
	var v string
	v, e = c.Get("a")
	if e != nil {
		t.Errorf(err)
	}
	if v != "b" {
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

	// Test improper initialization
	c := &Context{}
	e := c.SetMulti("a", nil)
	if e != ErrContextNotInitialized {
		t.Errorf(err)
	}

	// Test set multi
	m := make([]string, 2)
	m[0] = "b"
	m[1] = "c"
	r, _ := http.NewRequest("GET", "http://test.com", nil)
	c = &Context{Request: r}
	e = c.SetMulti("a", m)
	if e != nil {
		t.Errorf(err)
	}
	var v []string
	v, e = c.GetMulti("a")
	if e != nil {
		t.Errorf(err)
	}
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
	c := &Context{Request: r}
	e := c.SetBool("a", true)
	if e != nil {
		t.Errorf(err)
	}
	var v bool
	v, e = c.GetBool("a")
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
	c := &Context{Request: r}
	e := c.SetFloat64("a", 1.5, 'G', -1)
	if e != nil {
		t.Errorf(err)
	}
	var v float64
	v, e = c.GetFloat64("a")
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
	c := &Context{Request: r}
	e := c.SetInt64("a", 1)
	if e != nil {
		t.Errorf(err)
	}
	var v int64
	v, e = c.GetInt64("a")
	if e != nil {
		t.Errorf(err)
	}
	if v != 1 {
		t.Errorf(err)
	}
}
