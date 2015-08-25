// matcher_test
package mux

import (
	"testing"
)

func TestMatcherAdd(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	m := &matcher{}
	a := &endpoint{}
	b := &endpoint{}
	c := &endpoint{}

	// Test add to root
	err := "Failed add to root."
	m.add("", a)
	v := m.root.data
	if v != a {
		t.Errorf(err)
	}

	// Test add child
	err = "Failed add child."
	m.add("child", a)
	v = m.root.children["child"].data
	if v != a {
		t.Errorf(err)
	}

	// Test add multiple children
	err = "Failed add multiple children."
	m.add("child/child2", b)
	v = m.root.children["child"].children["child2"].data
	if v != b {
		t.Errorf(err)
	}

	m.add("child3/child4", c)
	v = m.root.children["child3"].children["child4"].data
	if v != c {
		t.Errorf(err)
	}

	// Test add wildcard
	err = "Failed add wildcard."
	m.add("{wc}", a)

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
	m.add("{wc: ^[0-9]+$}", b)

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

func TestMatcherMatch(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	m := &matcher{}
	a := &endpoint{}
	b := &endpoint{}
	c := &endpoint{}
	d := &endpoint{}
	f := &endpoint{}
	g := &endpoint{}
	h := &endpoint{}

	// Test match non-existent
	err := "Failed match non-existent."
	_, e := m.match("non-existent")
	if e != ErrNotFound {
		t.Errorf(err)
	}

	// Test match root
	err = "Failed match root."
	m.add("", a)
	results, e := m.match("")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.data() != a {
		t.Errorf(err)
	}

	// Test match child
	err = "Failed match child."
	m.add("child", a)
	results, e = m.match("child")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.data() != a {
		t.Errorf(err)
	}

	// Test match multiple children
	err = "Failed match multiple children."
	m.add("child/child2", b)
	results, e = m.match("child/child2")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.data() != b {
		t.Errorf(err)
	}

	m.add("child3/child4", c)
	results, e = m.match("child3/child4")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.data() != c {
		t.Errorf(err)
	}

	// Test match trailing slash
	err = "Failed match trailing slash."
	m.add("match", d)
	_, e = m.match("match/")
	if e != ErrRedirectSlash {
		t.Errorf(err)
	}

	m.add("match2/", f)
	_, e = m.match("match2")
	if e != ErrRedirectSlash {
		t.Errorf(err)
	}

	// Test match wildcard
	err = "Failed match wildcard."
	m.add("{wc}", g)
	results, e = m.match("test")
	if e != nil {
		t.Errorf(e.Error())
	}
	found := false
	for _, v := range results.params() {
		if v.key == "wc" && v.value == "test" {
			found = true
		}
	}
	if !found {
		t.Errorf(err)
	}
	if results.data() != g {
		t.Errorf(err)
	}

	// Test match wildcard with regex
	err = "Failed match wildcard with regex."
	m.add("{wc: ^[0-9]+$}", h)
	_, e = m.match("test")
	if e != ErrNotFound {
		t.Errorf(err)
	}
	results, e = m.match("42")
	if e != nil {
		t.Errorf(e.Error())
	}
	found = false
	for _, v := range results.params() {
		if v.key == "wc" && v.value == "42" {
			found = true
		}
	}
	if !found {
		t.Errorf(err)
	}
	if results.data() != h {
		t.Errorf(err)
	}

	// Test explicit match
	_, e = m.matchExplicit("test")
	if e == nil {
		t.Errorf(err)
	}
	results, e = m.matchExplicit("{test}")
	if e != nil {
		t.Errorf(e.Error())
	}
	found = false
	for _, v := range results.params() {
		if v.key == "wc" && v.value == "{test}" {
			found = true
		}
	}
	if !found {
		t.Errorf(err)
	}
	if results.data() != h {
		t.Errorf(err)
	}
}
