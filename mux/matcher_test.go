// matcher_test
package mux

import (
	"testing"
)

func TestNodeAdd(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	n := newMatcherNode()

	// Test add to root
	err := "Failed add to root."
	p := make([]string, 0)
	n.add(p, "A")
	v := n.data
	if v != "A" {
		t.Errorf(err)
	}

	// Test add to child
	err = "Failed add to child."
	p = []string{"child"}
	n.add(p, "A")
	v = n.children["child"].data
	if v != "A" {
		t.Errorf(err)
	}

	// Test add multiple children
	err = "Failed add multiple children."
	p = []string{"child", "child2"}
	n.add(p, "B")
	v = n.children["child"].children["child2"].data
	if v != "B" {
		t.Errorf(err)
	}

	p = []string{"child3", "child4"}
	n.add(p, "C")
	v = n.children["child3"].children["child4"].data
	if v != "C" {
		t.Errorf(err)
	}

	// Test add wildcard
	err = "Failed add wildcard."
	p = []string{"{wc}"}
	n.add(p, "A")

	nChild, ok := n.children["*"]
	if !ok {
		t.Errorf(err)
	}

	if nChild.wildcard != "wc" {
		t.Errorf(err)
	}
	v = nChild.data
	if v != "A" {
		t.Errorf(err)
	}

	// Test add to wildcard with regex
	err = "Failed add to wildcard with regex."
	p = []string{"{wc:^[0-9]+}"}
	n.add(p, "B")

	nChild, ok = n.children["*"]
	if !ok {
		t.Errorf(err)
	}

	if nChild.wildcard != "wc" {
		t.Errorf(err)
	}
	if !nChild.regex.MatchString("42") {
		t.Errorf(err)
	}
	v = nChild.data
	if v != "B" {
		t.Errorf(err)
	}
}

func TestNodeMatch(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	n := newMatcherNode()

	// Test match non-existent
	err := "Failed match non-existent."
	_, e := n.match([]string{"non-existent"})
	if e != ErrNotFound {
		t.Errorf(err)
	}

	// Test match root
	err = "Failed match root."
	p := make([]string, 0)
	n.add(p, "A")
	results, e := n.match(p)
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != "A" {
		t.Errorf(err)
	}

	// Test match child
	err = "Failed match child."
	p = []string{"child"}
	n.add(p, "A")
	results, e = n.match(p)
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != "A" {
		t.Errorf(err)
	}

	// Test match multiple children
	err = "Failed match multiple children."
	p = []string{"child", "child2"}
	n.add(p, "B")
	results, e = n.match(p)
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != "B" {
		t.Errorf(e.Error())
	}

	p = []string{"child3", "child4"}
	n.add(p, "C")
	results, e = n.match(p)
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != "C" {
		t.Errorf(err)
	}

	// Test match wildcard
	err = "Failed match wildcard."
	p = []string{"{wc}"}
	n.add(p, "A")
	results, e = n.match([]string{"test"})
	if e != nil {
		t.Errorf(e.Error())
	}
	if param := results.Values().Get("wc"); param != "test" {
		t.Errorf(err)
	}
	if results.Data() != "A" {
		t.Errorf(err)
	}

	// Test match wildcard with regex
	err = "Failed match wildcard with regex."
	p = []string{"{wc: ^[0-9]+$}"}
	n.add(p, "B")
	_, e = n.match([]string{"test"})
	if e != ErrNotFound {
		t.Errorf(err)
	}
	results, e = n.match([]string{"42"})
	if e != nil {
		t.Errorf(e.Error())
	}
	if param := results.Values().Get("wc"); param != "42" {
		t.Errorf(err)
	}
	if results.Data() != "B" {
		t.Errorf(err)
	}
}

func TestNodeMatchPrefix(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	n := newMatcherNode()

	// Test match root
	err := "Failed match root."
	p := make([]string, 0)
	n.add(p, "A")
	results := n.matchPrefix(p)
	if results.Data() != "A" {
		t.Errorf(err)
	}

	results = n.matchPrefix([]string{"path"})
	if results.Data() != "A" {
		t.Errorf(err)
	}

	// Test match child
	err = "Failed match child."
	p = []string{"child"}
	n.add(p, "B")
	results = n.matchPrefix(p)
	if results.Data() != "B" {
		t.Errorf(err)
	}
	results = n.matchPrefix([]string{"child", "child2"})
	if results.Data() != "B" {
		t.Errorf(err)
	}
	results = n.matchPrefix([]string{"non-existent"})
	if results.Data() != "A" {
		t.Errorf(err)
	}

	// Test match multiple children
	err = "Failed match multiple children."
	p = []string{"child", "child2"}
	n.add(p, "C")
	results = n.matchPrefix(p)
	if results.Data() != "C" {
		t.Errorf(err)
	}
	results = n.matchPrefix([]string{"child", "child2", "child3"})
	if results.Data() != "C" {
		t.Errorf(err)
	}
	results = n.matchPrefix([]string{"child", "child4", "child3"})
	if results.Data() != "B" {
		t.Errorf(err)
	}
	results = n.matchPrefix([]string{"child5"})
	if results.Data() != "A" {
		t.Errorf(err)
	}
}

func TestDefaultMatcherAdd(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	m := &DefaultMatcher{}

	// Test add to root
	err := "Failed add to root."
	m.Add("", "A")
	v := m.root.data
	if v != "A" {
		t.Errorf(err)
	}

	// Test add child
	err = "Failed add child."
	m.Add("child", "A")
	v = m.root.children["child"].data
	if v != "A" {
		t.Errorf(err)
	}

	// Test add multiple children
	err = "Failed add multiple children."
	m.Add("child/child2", "B")
	v = m.root.children["child"].children["child2"].data
	if v != "B" {
		t.Errorf(err)
	}

	m.Add("child3/child4", "C")
	v = m.root.children["child3"].children["child4"].data
	if v != "C" {
		t.Errorf(err)
	}

	// Test add wildcard
	err = "Failed add wildcard."
	m.Add("{wc}", "A")

	nChild, ok := m.root.children["*"]
	if !ok {
		t.Errorf(err)
	}

	if nChild.wildcard != "wc" {
		t.Errorf(err)
	}
	v = nChild.data
	if v != "A" {
		t.Errorf(err)
	}

	// Test add wildcard with regex
	err = "Failed add wildcard with regex."
	m.Add("{wc: ^[0-9]+$}", "B")

	nChild, ok = m.root.children["*"]
	if !ok {
		t.Errorf(err)
	}

	if nChild.wildcard != "wc" {
		t.Errorf(err)
	}
	if !nChild.regex.MatchString("42") {
		t.Errorf(err)
	}
	v = nChild.data
	if v != "B" {
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

	// Test match non-existent
	err := "Failed match non-existent."
	_, e := m.Match("non-existent")
	if e != ErrNotFound {
		t.Errorf(err)
	}

	// Test match root
	err = "Failed match root."
	m.Add("", "A")
	results, e := m.Match("")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != "A" {
		t.Errorf(err)
	}

	// Test match child
	err = "Failed match child."
	m.Add("child", "A")
	results, e = m.Match("child")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != "A" {
		t.Errorf(err)
	}

	// Test match multiple children
	err = "Failed match multiple children."
	m.Add("child/child2", "B")
	results, e = m.Match("child/child2")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != "B" {
		t.Errorf(err)
	}

	m.Add("child3/child4", "C")
	results, e = m.Match("child3/child4")
	if e != nil {
		t.Errorf(e.Error())
	}
	if results.Data() != "C" {
		t.Errorf(err)
	}

	// Test match trailing slash
	err = "Failed match trailing slash."
	m.Add("match", "E")
	_, e = m.Match("match/")
	if e != ErrRedirectSlash {
		t.Errorf(err)
	}

	m.Add("match2/", "F")
	_, e = m.Match("match2")
	if e != ErrRedirectSlash {
		t.Errorf(err)
	}

	// Test match wildcard
	err = "Failed match wildcard."
	m.Add("{wc}", "G")
	results, e = m.Match("test")
	if e != nil {
		t.Errorf(e.Error())
	}
	if param := results.Values().Get("wc"); param != "test" {
		t.Errorf(err)
	}
	if results.Data() != "G" {
		t.Errorf(err)
	}

	// Test match wildcard with regex
	err = "Failed match wildcard with regex."
	m.Add("{wc: ^[0-9]+$}", "H")
	_, e = m.Match("test")
	if e != ErrNotFound {
		t.Errorf(err)
	}
	results, e = m.Match("42")
	if e != nil {
		t.Errorf(e.Error())
	}
	if param := results.Values().Get("wc"); param != "42" {
		t.Errorf(err)
	}
	if results.Data() != "H" {
		t.Errorf(err)
	}
}

func TestDefaultMatcherMatchPrefix(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

}
