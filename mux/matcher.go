// Matcher
package mux

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// --------------------------------
// ---------- Mux Errors ----------

var ErrNotFound = errors.New("mux: Handler not found.")
var ErrNotImplemented = errors.New("mux: Handler not implemented.")

// -----------------------------
// ---------- Matcher ----------

const wcStr string = "*"

// Interface for a matcher that matches method+paths to objects.
type Matcher interface {
	// Add an object to the matcher registered to the method and path.
	Add(method, path string, object interface{})

	// Attempt to match a method and patch to an object.
	Match(method, path string) (interface{}, url.Values, error)
}

type matcherNode struct {
	data     map[string]interface{}
	children map[string]*matcherNode

	wildcard string
	regex    *regexp.Regexp
}

func newMatcherNode() *matcherNode {
	return &matcherNode{
		data:     make(map[string]interface{}),
		children: make(map[string]*matcherNode),
	}
}

// Add an object to a nodes data map by recursively traversing the path
// and node tree.
func (n *matcherNode) add(method string, path []string, object interface{}) {
	if len(path) == 0 {
		// End of path, attach object to method for node.

		n.data[method] = object
		return
	}

	part := path[0]
	if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
		// Path segment is wildcard, handle wildcard case

		child, exists := n.children[wcStr]
		if !exists {
			child = newMatcherNode()
			n.children[wcStr] = child
		}

		expression := strings.TrimPrefix(strings.TrimSuffix(part, "}"), "{")
		expression = strings.TrimSpace(expression)

		if strings.Contains(expression, ":") {
			// Path segment contains regexp, compile regexp matcher for future matching

			exp_split := strings.Split(expression, ":")
			expression = strings.TrimSpace(exp_split[0])
			regex := strings.TrimSpace(exp_split[1])

			var err error
			child.regex, err = regexp.Compile(regex)
			if err != nil {
				panic("Could not compile: " + err.Error())
			}
		}

		child.wildcard = expression
		child.add(method, path[1:], object)

	} else {
		// Regular case, recursively add object to method

		child, exists := n.children[part]
		if !exists {
			child = newMatcherNode()
			n.children[part] = child
		}
		child.add(method, path[1:], object)
	}
}

// Attempt to match a method and path to an object by recursively traversing
// the path and node tree.
func (n *matcherNode) match(method string, path []string) (interface{}, url.Values, error) {
	if len(path) == 0 {
		// End of path, attempt to find object attached to method and return it.

		data, ok := n.data[method]
		if !ok {
			return nil, nil, ErrNotImplemented
		}

		return data, url.Values{}, nil
	}

	part := path[0]
	child, exists := n.children[part]
	if !exists {
		// Path segment not found, check for wildcard.

		child, exists := n.children[wcStr]
		if !exists {
			// No wildcard

			return nil, nil, ErrNotFound
		}
		if child.regex != nil && !child.regex.MatchString(part) {
			// Regexp exists but segment does not match.

			return nil, nil, ErrNotFound
		}

		data, values, err := child.match(method, path[1:])
		if err != nil {
			return nil, nil, err
		}

		values.Set(child.wildcard, part)

		return data, values, nil
	} else {
		// Regular case, continue recursively traversing tree

		return child.match(method, path[1:])
	}
}

// Default Matcher implementation.
type DefaultMatcher struct {
	root *matcherNode
}

func (m *DefaultMatcher) Add(method, path string, object interface{}) {
	if m.root == nil {
		m.root = newMatcherNode()
	}

	path_split := strings.Split(path, "/")
	if len(path_split[0]) == 0 {
		path_split = path_split[1:]
	}
	m.root.add(method, path_split, object)
}

func (m *DefaultMatcher) Match(method, path string) (interface{}, url.Values, error) {
	if m.root == nil {
		return nil, nil, ErrNotFound
	}

	path_split := strings.Split(path, "/")
	if len(path_split[0]) == 0 {
		path_split = path_split[1:]
	}
	return m.root.match(method, path_split)
}
