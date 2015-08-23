// matcher_test
package mux

import (
	"testing"
)

func TestDefaultMatcherAdd(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	m := &DefaultMatcher{}
	a := &endpoint{}
	b := &endpoint{}
	c := &endpoint{}

	// Test add to root
	err := "Failed add to root."
	m.Add("", a)
	v := m.root.data
	if v != a {
		t.Errorf(err)
	}

	// Test add child
	err = "Failed add child."
	m.Add("child", a)
	v = m.root.children["child"].data
	if v != a {
		t.Errorf(err)
	}

	// Test add multiple children
	err = "Failed add multiple children."
	m.Add("child/child2", b)
	v = m.root.children["child"].children["child2"].data
	if v != b {
		t.Errorf(err)
	}

	m.Add("child3/child4", c)
	v = m.root.children["child3"].children["child4"].data
	if v != c {
		t.Errorf(err)
	}

	// Test add wildcard
	err = "Failed add wildcard."
	m.Add("{wc}", a)

	nChild := m.root.wildChild
	if nChild == nil {
		t.Errorf(err)
	}

	if nChild.wildcard != "wc" {
		t.Errorf(err)
	}
	v = nChild.data
	if v != a {
		t.Errorf(err)
	}

	// Test add wildcard with regex
	err = "Failed add wildcard with regex."
	m.Add("{wc: ^[0-9]+$}", b)

	nChild = m.root.wildChild
	if nChild == nil {
		t.Errorf(err)
	}

	if nChild.wildcard != "wc" {
		t.Errorf(err)
	}
	if !nChild.regex.MatchString("42") {
		t.Errorf(err)
	}
	v = nChild.data
	if v != b {
		t.Errorf(err)
	}
}

func TestDefaultMatcherMatch(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	m := &DefaultMatcher{}
	a := &endpoint{}
	b := &endpoint{}
	c := &endpoint{}
	d := &endpoint{}
	f := &endpoint{}
	g := &endpoint{}
	h := &endpoint{}

	// Test match non-existent
	err := "Failed match non-existent."
	_, e := m.Match("non-existent")
	if e != ErrNotFound {
		t.Errorf(err)
	}

	// Test match root
	err = "Failed match root."
	m.Add("", a)
	results, e := m.Match("")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != a {
		t.Errorf(err)
	}

	// Test match child
	err = "Failed match child."
	m.Add("child", a)
	results, e = m.Match("child")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != a {
		t.Errorf(err)
	}

	// Test match multiple children
	err = "Failed match multiple children."
	m.Add("child/child2", b)
	results, e = m.Match("child/child2")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != b {
		t.Errorf(err)
	}

	m.Add("child3/child4", c)
	results, e = m.Match("child3/child4")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != c {
		t.Errorf(err)
	}

	// Test match trailing slash
	err = "Failed match trailing slash."
	m.Add("match", d)
	_, e = m.Match("match/")
	if e != ErrRedirectSlash {
		t.Errorf(err)
	}

	m.Add("match2/", f)
	_, e = m.Match("match2")
	if e != ErrRedirectSlash {
		t.Errorf(err)
	}

	// Test match wildcard
	err = "Failed match wildcard."
	m.Add("{wc}", g)
	results, e = m.Match("test")
	if e != nil {
		t.Errorf(e.Error())
	}
	found := false
	for _, v := range results.Params() {
		if v.Key == "wc" && v.Value == "test" {
			found = true
		}
	}
	if !found {
		t.Errorf(err)
	}
	if results.Data() != g {
		t.Errorf(err)
	}

	// Test match wildcard with regex
	err = "Failed match wildcard with regex."
	m.Add("{wc: ^[0-9]+$}", h)
	_, e = m.Match("test")
	if e != ErrNotFound {
		t.Errorf(err)
	}
	results, e = m.Match("42")
	if e != nil {
		t.Errorf(e.Error())
	}
	found = false
	for _, v := range results.Params() {
		if v.Key == "wc" && v.Value == "42" {
			found = true
		}
	}
	if !found {
		t.Errorf(err)
	}
	if results.Data() != h {
		t.Errorf(err)
	}

	// Test ignore regex
	results, e = m.MatchNoRegex("test")
	if e != nil {
		t.Errorf(e.Error())
	}
	found = false
	for _, v := range results.Params() {
		if v.Key == "wc" && v.Value == "test" {
			found = true
		}
	}
	if !found {
		t.Errorf(err)
	}
	if results.Data() != h {
		t.Errorf(err)
	}
}
