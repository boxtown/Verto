package mux

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// --------------------------------
// ---------- Mux Errors ----------

// ErrNotFound gets returned by Matcher if a path could not be matched.
var ErrNotFound = errors.New("mux: handler not found")

// ErrNotImplemented gets returned by Matcher if a path could be matched
// but the method could not be found.
var ErrNotImplemented = errors.New("mux: handler not implemented")

// ErrRedirectSlash gets returned by Matcher if a path could not be matched
// but a path with (without) a slash exists.
var ErrRedirectSlash = errors.New("mux: redirect trailing slash")

// -----------------------------
// ---------- Matcher ----------

const wcStr string = "*"

// Interface for returning results from the matcher
type Results interface {
	// Returns the resulting data from the path match
	Data() interface{}

	// Returns any path parameters encountered while searching.
	Values() url.Values
}

// Interface for a matcher that matches paths to objects.
type Matcher interface {
	// Add an object to the matcher registered to the path.
	Add(path string, object interface{})

	// Deletes the object stored at the node at the path
	// if it exists.
	Delete(path string)

	// Returns the object stored at the node with the longest
	// prefix match.
	LongestPrefixMatch(path string) Results

	// Attempt to match a path to an object.
	Match(path string) (Results, error)

	// Returns all non-nil objects that explicitly matches
	// the prefix.
	PrefixMatch(prefix string) []interface{}
}

type matcherResults struct {
	data   interface{}
	values url.Values
}

func (mr *matcherResults) Data() interface{} {
	return mr.data
}

func (mr *matcherResults) Values() url.Values {
	return mr.values
}

type matcherNode struct {
	data     interface{}
	parent   *matcherNode
	children map[string]*matcherNode

	wildcard string
	regex    *regexp.Regexp
}

func newMatcherNode() *matcherNode {
	return &matcherNode{
		children: make(map[string]*matcherNode),
	}
}

// Add an object to a nodes data map by traversing the node tree.
func (n *matcherNode) add(path []string, object interface{}) {

	node := n

	for i := 0; i < len(path); i++ {
		segment := path[i]
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			// Path segment is wildcard

			child, exists := node.children[wcStr]
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
			node = child
		} else {
			// Regular case

			child, exists := node.children[segment]
			if !exists {
				child = newMatcherNode()
				child.parent = node
				node.children[segment] = child
			}
			node = child
		}
	}

	node.data = object
}

// Deletes the data for the node at path if it exists.
func (n *matcherNode) delete(path []string) {

	node := n
	for i := range path {
		segment := path[i]
		child, exists := node.children[segment]
		if !exists {
			return
		}
		node = child
	}

	node.data = nil
}

// Returns the results from the longest common prefix match
func (n *matcherNode) longestPrefixMatch(path []string) Results {

	node := n

	results := &matcherResults{values: url.Values{}}

	for i := range path {
		segment := path[i]
		child, exists := node.children[segment]
		if !exists {

			child, exists = node.children[wcStr]
			if !exists {
				results.data = node.data
				return results
			}
			if child.regex != nil && !child.regex.MatchString(segment) {
				results.data = node.data
				return results
			}

			results.values.Add(child.wildcard, segment)
		}

		node = child
	}

	results.data = node.data
	return results
}

// Attempt to match a path to an object by traversing
// the node tree.
func (n *matcherNode) match(path []string) (Results, error) {

	node := n
	results := &matcherResults{values: url.Values{}}

	for i := 0; i < len(path); i++ {
		segment := path[i]
		child, exists := node.children[segment]
		if !exists {
			// Path segment not found, check for wildcard or trailing slash

			if i > 0 && i == len(path)-1 && segment == "" {
				// Handle trailing slash
				redirect, exists := node.parent.children[path[i-1]]
				if !exists {
					redirect, exists = node.parent.children[wcStr]
				}

				if exists {
					if redirect.data != nil {
						return nil, ErrRedirectSlash
					}
				}

				return nil, ErrNotFound
			}

			child, exists = node.children[wcStr]
			if !exists {
				// No wildcard

				return nil, ErrNotFound
			}
			if child.regex != nil && !child.regex.MatchString(segment) {
				// Regex but segment doesn't match.

				return nil, ErrNotFound
			}
			results.values.Add(child.wildcard, segment)
		}
		node = child
	}

	if node.data == nil {
		if child, exists := node.children[""]; exists {
			if child.data != nil {
				return nil, ErrRedirectSlash
			}
		}

		return nil, ErrNotFound
	}

	results.data = node.data
	return results, nil
}

// Return non-nil data of any nodes explicitly matching the prefix.
func (n *matcherNode) prefixMatch(prefix []string) []interface{} {
	results := make([]interface{}, 0)

	node := n
	for i := range prefix {
		segment := prefix[i]
		child, exists := node.children[segment]
		if !exists {
			break
		}
		node = child
	}

	if node != n && len(node.children) > 0 {
		if node.data != nil {
			results = append(results, node.data)
		}

		queue := make([]*matcherNode, 0)
		for _, v := range node.children {
			queue = append(queue, v)
		}

		for len(queue) > 0 {
			node = queue[0]
			queue = queue[1:]

			if node.data != nil {
				results = append(results, node.data)
			}

			for _, v := range node.children {
				queue = append(queue, v)
			}
		}
	}

	return results
}

// Default Matcher implementation.
type DefaultMatcher struct {
	root *matcherNode
}

func (m *DefaultMatcher) Add(path string, object interface{}) {
	if m.root == nil {
		m.root = newMatcherNode()
	}

	pathSplit := m.splitPath(path)
	m.root.add(pathSplit, object)
}

func (m *DefaultMatcher) Delete(path string) {
	if m.root == nil {
		return
	}

	pathSplit := m.splitPath(path)
	m.root.delete(pathSplit)
}

func (m *DefaultMatcher) LongestPrefixMatch(path string) Results {
	if m.root == nil {
		return &matcherResults{}
	}

	pathSplit := m.splitPath(path)
	return m.root.longestPrefixMatch(pathSplit)
}

func (m *DefaultMatcher) Match(path string) (Results, error) {
	if m.root == nil {
		return nil, ErrNotFound
	}

	pathSplit := m.splitPath(path)
	return m.root.match(pathSplit)
}

func (m *DefaultMatcher) PrefixMatch(prefix string) []interface{} {
	if m.root == nil {
		return make([]interface{}, 0)
	}

	prefixSplit := m.splitPath(prefix)
	return m.root.prefixMatch(prefixSplit)
}

func (m *DefaultMatcher) splitPath(path string) []string {
	pathSplit := strings.Split(path, "/")
	if len(pathSplit) > 0 && len(pathSplit[0]) == 0 {
		pathSplit = pathSplit[1:]
	}
	return pathSplit
}
