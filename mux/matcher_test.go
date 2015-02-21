// matcher_test
package mux

import (
	"net/url"
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
	n.add("GET", p, "A")
	obj := n.data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "A" {
		t.Errorf(err)
	}

	// Test add to child
	err = "Failed add to child."
	p = []string{"child"}
	n.add("GET", p, "A")
	obj = n.children["child"].data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "A" {
		t.Errorf(err)
	}

	// Test add multiple children
	err = "Failed add multiple children."
	p = []string{"child", "child2"}
	n.add("GET", p, "B")
	obj = n.children["child"].children["child2"].data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "B" {
		t.Errorf(err)
	}

	p = []string{"child3", "child4"}
	n.add("GET", p, "C")
	obj = n.children["child3"].children["child4"].data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "C" {
		t.Errorf(err)
	}

	// Test add wildcard
	err = "Failed add wildcard."
	p = []string{"{wc}"}
	n.add("GET", p, "A")

	nChild, ok := n.children["*"]
	if !ok {
		t.Errorf(err)
	}

	if nChild.wildcard != "wc" {
		t.Errorf(err)
	}
	obj = nChild.data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "A" {
		t.Errorf(err)
	}

	// Test add to wildcard with regex
	err = "Failed add to wildcard with regex."
	p = []string{"{wc:^[0-9]+}"}
	n.add("GET", p, "B")

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
	obj = nChild.data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "B" {
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
	_, _, e := n.match("GET", []string{"non-existent"})
	if e != ErrNotFound {
		t.Errorf(err)
	}

	// Test match root
	err = "Failed match root."
	p := make([]string, 0)
	n.add("GET", p, "A")
	obj, _, e := n.match("GET", p)
	if e != nil {
		t.Errorf(e.Error())
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "A" {
		t.Errorf(err)
	}

	// Test match child
	err = "Failed match child."
	p = []string{"child"}
	n.add("GET", p, "A")
	obj, _, e = n.match("GET", p)
	if e != nil {
		t.Errorf(e.Error())
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "A" {
		t.Errorf(err)
	}

	// Test match multiple children
	err = "Failed match multiple children."
	p = []string{"child", "child2"}
	n.add("GET", p, "B")
	obj, _, e = n.match("GET", p)
	if e != nil {
		t.Errorf(e.Error())
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "B" {
		t.Errorf(e.Error())
	}

	p = []string{"child3", "child4"}
	n.add("GET", p, "C")
	obj, _, e = n.match("GET", p)
	if e != nil {
		t.Errorf(e.Error())
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "C" {
		t.Errorf(err)
	}

	// Test match wildcard
	var params url.Values
	err = "Failed match wildcard."
	p = []string{"{wc}"}
	n.add("GET", p, "A")
	obj, params, e = n.match("GET", []string{"test"})
	if e != nil {
		t.Errorf(e.Error())
	}
	if param := params.Get("wc"); len(param) == 0 {
		t.Errorf(err)
	} else if param != "test" {
		t.Errorf(err)
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "A" {
		t.Errorf(err)
	}

	// Test match wildcard with regex
	err = "Failed match wildcard with regex."
	p = []string{"{wc: ^[0-9]+$}"}
	n.add("GET", p, "B")
	_, _, e = n.match("GET", []string{"test"})
	if e != ErrNotFound {
		t.Errorf(err)
	}
	obj, params, e = n.match("GET", []string{"42"})
	if e != nil {
		t.Errorf(e.Error())
	}
	if param := params.Get("wc"); len(params) == 0 {
		t.Errorf(err)
	} else if param != "42" {
		t.Errorf(err)
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "B" {
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
	m.Add("GET", "", "A")
	obj := m.root.data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "A" {
		t.Errorf(err)
	}

	// Test add child
	err = "Failed add child."
	m.Add("GET", "child", "A")
	obj = m.root.children["child"].data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "A" {
		t.Errorf(err)
	}

	// Test add multiple children
	err = "Failed add multiple children."
	m.Add("GET", "child/child2", "B")
	obj = m.root.children["child"].children["child2"].data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "B" {
		t.Errorf(err)
	}

	m.Add("GET", "child3/child4", "C")
	obj = m.root.children["child3"].children["child4"].data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "C" {
		t.Errorf(err)
	}

	// Test add wildcard
	err = "Failed add wildcard."
	m.Add("GET", "{wc}", "A")

	nChild, ok := m.root.children["*"]
	if !ok {
		t.Errorf(err)
	}

	if nChild.wildcard != "wc" {
		t.Errorf(err)
	}
	obj = nChild.data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "A" {
		t.Errorf(err)
	}

	// Test add wildcard with regex
	err = "Failed add wildcard with regex."
	m.Add("GET", "{wc: ^[0-9]+$}", "B")

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
	obj = nChild.data["GET"]
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "B" {
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
	_, _, e := m.Match("GET", "non-existent")
	if e != ErrNotFound {
		t.Errorf(err)
	}

	// Test match root
	err = "Failed match root."
	m.Add("GET", "", "A")
	obj, _, e := m.Match("GET", "")
	if e != nil {
		t.Errorf(e.Error())
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "A" {
		t.Errorf(err)
	}

	// Test match child
	err = "Failed match child."
	m.Add("GET", "child", "A")
	obj, _, e = m.Match("GET", "child")
	if e != nil {
		t.Errorf(e.Error())
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "A" {
		t.Errorf(err)
	}

	// Test match multiple children
	err = "Failed match multiple children."
	m.Add("GET", "child/child2", "B")
	obj, _, e = m.Match("GET", "child/child2")
	if e != nil {
		t.Errorf(e.Error())
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "B" {
		t.Errorf(err)
	}

	m.Add("GET", "child3/child4", "C")
	obj, _, e = m.Match("GET", "child3/child4")
	if e != nil {
		t.Errorf(e.Error())
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "C" {
		t.Errorf(err)
	}

	// Test match wildcard
	var params url.Values
	err = "Failed match wildcard."
	m.Add("GET", "{wc}", "D")
	obj, params, e = m.Match("GET", "test")
	if e != nil {
		t.Errorf(e.Error())
	}
	if param := params.Get("wc"); len(param) == 0 {
		t.Errorf(err)
	} else if param != "test" {
		t.Errorf(err)
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "D" {
		t.Errorf(err)
	}

	// Test match wildcard with regex
	err = "Failed match wildcard with regex."
	m.Add("GET", "{wc: ^[0-9]+$}", "E")
	_, _, e = m.Match("GET", "test")
	if e != ErrNotFound {
		t.Errorf(err)
	}
	obj, params, e = m.Match("GET", "42")
	if e != nil {
		t.Errorf(e.Error())
	}
	if param := params.Get("wc"); len(param) == 0 {
		t.Errorf(err)
	} else if param != "42" {
		t.Errorf(err)
	}
	if v, ok := obj.(string); !ok {
		t.Errorf(err)
	} else if v != "E" {
		t.Errorf(err)
	}
}
