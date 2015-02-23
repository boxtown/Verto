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
var ErrRedirectSlash = errors.New("mux: Redirect trailing slash")

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
	parent   *matcherNode
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

	node := n
	child := node
	exists := false

	for i := 0; i < len(path); i++ {
		segment := path[i]
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			// Path segment is wildcard

			child, exists = node.children[wcStr]
			if !exists {
				child = newMatcherNode()
				child.parent = node
				node.children[wcStr] = child
			}

			expression := strings.TrimPrefix(strings.TrimSuffix(segment, "}"), "{")
			expression = strings.TrimSpace(expression)

			if strings.Contains(expression, ":") {
				// Path segment contains regexp

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
		} else {
			// Regular case

			child, exists = node.children[segment]
			if !exists {
				child = newMatcherNode()
				child.parent = node
				node.children[segment] = child
			}
		}

		node = child
	}

	node.data[method] = object
}

// Attempt to match a method and path to an object by recursively traversing
// the path and node tree.
func (n *matcherNode) match(method string, path []string) (interface{}, url.Values, error) {

	node := n
	child := node
	exists := false
	params := url.Values{}

	for i := 0; i < len(path); i++ {
		segment := path[i]
		child, exists = node.children[segment]
		if !exists {
			// Path segment not found, check for wildcard or trailing slash

			if i > 0 && i == len(path)-1 && segment == "" {
				// Handle trailing slash

				if _, exists := node.parent.children[path[i-1]].data[method]; exists {
					return nil, nil, ErrRedirectSlash
				}
				return nil, nil, ErrNotFound
			}

			child, exists = node.children[wcStr]
			if !exists {
				// No wildcard

				return nil, nil, ErrNotFound
			}
			if child.regex != nil && !child.regex.MatchString(segment) {
				// Regex but segment doesn't match.

				return nil, nil, ErrNotFound
			}

			params.Add(child.wildcard, segment)
		}

		node = child
	}

	data, ok := node.data[method]
	if !ok {
		if child, exists := node.children[""]; exists {
			if _, exists = child.data[method]; exists {
				return nil, nil, ErrRedirectSlash
			}
		}

		return nil, nil, ErrNotImplemented
	}
	return data, params, nil
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

	// Need this because splitting an empty string
	// returns a len 1 slice for some reason
	if len(path_split) > 0 && len(path_split[0]) == 0 {
		path_split = path_split[1:]
	}

	m.root.add(method, path_split, object)
}

func (m *DefaultMatcher) Match(method, path string) (interface{}, url.Values, error) {
	if m.root == nil {
		return nil, nil, ErrNotFound
	}

	path_split := strings.Split(path, "/")

	// Need this because splitting an empty string
	// returns a len 1 slice for some reason
	if len(path_split) > 0 && len(path_split[0]) == 0 {
		path_split = path_split[1:]
	}

	return m.root.match(method, path_split)
}
