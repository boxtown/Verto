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
const catchAll string = "^"
const empty string = ""

// Param represents a Key-Value HTTP parameter pair
type Param struct {
	Key   string
	Value string
}

// Results is an interface for returning results from the matcher
type Results interface {
	// Returns the resulting data from the path match
	Data() interface{}

	// Returns all parameter objects as a slice
	Params() []Param

	// Converts parameters slice into a values map and returns it
	Values() url.Values
}

// Matcher is an interface for a matcher that matches paths to objects.
// { } denotes a wildcard segment. ^ denotes a catch-all segment.
type Matcher interface {
	// Add adds an object to the matcher registered to the path.
	Add(path string, object interface{})

	// Deletes the path but leaves sub-paths intact.
	Delete(path string)

	// Deletes the path and all sub-paths rooted at path.
	Drop(path string)

	// LongestPrefixMatch returns the object whose path
	// is the longest match with path. The longest possible
	// match is an object whose path is exactly path.
	LongestPrefixMatch(path string) Results

	// Match returns the the data and values associated with
	// the path or an error if the path isn't cannot be found.
	// Should return ErrRedirectSlash if a trailing slash redirect
	// is possible.
	Match(path string) (Results, error)

	// Returns the maximum possible number of wildcard parameters
	MaxParams() int

	// PrefixMatch returns all objects whose path contains prefix
	// as a prefix.
	PrefixMatch(prefix string) []interface{}
}

// Implements the Results interface
type matcherResults struct {
	data   interface{}
	values url.Values
	pairs  []Param
}

func newResults(maxParams int) *matcherResults {
	return &matcherResults{
		pairs: make([]Param, 0, maxParams),
	}
}

func (mr *matcherResults) addPair(key, value string) {
	i := len(mr.pairs)
	mr.pairs = mr.pairs[:i+1]
	mr.pairs[i].Key = key
	mr.pairs[i].Value = value
}

func (mr *matcherResults) Data() interface{} {
	return mr.data
}

func (mr *matcherResults) Params() []Param {
	return mr.pairs
}

func (mr *matcherResults) Values() url.Values {
	mr.values = url.Values{}
	for _, v := range mr.pairs {
		mr.values.Add(v.Key, v.Value)
	}
	return mr.values
}

// node used in matcher tree
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

// DefaultMatcher is the default implementation
// of the matcher interface.
type DefaultMatcher struct {
	root      *matcherNode
	maxParams int
}

// Add registers an object with a specific path. Wildcard path
// segments are denoted by {}'s. The string within the brackets is
// used as the key for key-value parameter pairs when matching a path.
// Regex can be defined inside wildcard path segments by appending a colon
// and a regex after the inner string. Catch-all paths are denoted with
// a '^'. Any path segments after a catch-all symbol are ignored as it
// does not make any sense to have child paths of a catch-all path.
func (m *DefaultMatcher) Add(path string, object interface{}) {
	if m.root == nil {
		m.root = newMatcherNode()
	}

	ps := m.splitPath(path)
	node := m.root
	nparams := 0
	for i := 0; i < len(ps); i++ {
		segment := ps[i]
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			// Path segment is wildcard
			child, exists := node.children[wcStr]
			if !exists {
				// If no wildcard node, create new one
				child = newMatcherNode()
				child.parent = node
				node.children[wcStr] = child
			}

			expression := strings.TrimPrefix(strings.TrimSuffix(segment, "}"), "{")
			expression = strings.TrimSpace(expression)
			if strings.Contains(expression, ":") {
				// Path segment contains regexp
				// Parse out and save regexp
				expSplit := strings.Split(expression, ":")
				expression = strings.TrimSpace(expSplit[0])
				regex := strings.TrimSpace(expSplit[1])

				var err error
				child.regex, err = regexp.Compile(regex)
				if err != nil {
					panic("Could not compile: " + err.Error())
				}
			}
			child.wildcard = expression
			node = child
			nparams++
		} else {
			// Get or add node for this segment and move on
			child, exists := node.children[segment]
			if !exists {
				child = newMatcherNode()
				child.parent = node
				node.children[segment] = child
			}
			if segment == catchAll {
				child.data = object
				return
			}
			node = child
		}
	}
	if nparams > m.maxParams {
		m.maxParams = nparams
	}
	node.data = object
}

// Delete nil's the value registered at path if such a path
// exists. Wildcard segments are not observed. Thus, to delete
// wildcard paths, a '*' must be used as the path segment.
// Catch-all segments must also be explicitly deleted using
// '^'.
func (m *DefaultMatcher) Delete(path string) {
	if m.root == nil {
		return
	}

	ps := m.splitPath(path)
	node := m.root
	for i := 0; i < len(ps); i++ {
		segment := ps[i]
		child, exists := node.children[segment]
		if !exists {
			return
		}
		node = child
		if segment == catchAll {
			break
		}
	}
	node.data = nil
}

// Drop deletes the path subtree rooted at path.
// Any paths with path as a prefix including the path
// at path are eliminated from the DefaultMatcher. Path
// matching works the same as it does in Delete where
// wildcards and catch-all's must be explicitly declared.
func (m *DefaultMatcher) Drop(path string) {
	if m.root == nil {
		return
	}

	ps := m.splitPath(path)
	node := m.root
	i := 0
	for ; i < len(ps); i++ {
		child, ok := node.children[ps[i]]
		if !ok {
			child, ok = node.children[wcStr]
			return
		}
		if ps[i] == catchAll {
			node = child
			break
		}
		node = child
	}
	if i > 0 {
		node = node.parent
		delete(node.children, ps[i-1])
	}
}

// LongestPrefixMatch returns the data in the registered with the longest prefix match with
// path. Wildcard segments are observed. LongestPrefixMatch at a minimum
// will always return data associated with the empty path.
func (m *DefaultMatcher) LongestPrefixMatch(path string) Results {
	if m.root == nil {
		return &matcherResults{}
	}

	ps := m.splitPath(path)
	node := m.root
	results := newResults(len(ps))
	var lastCatchAll *matcherNode

	for i := 0; i < len(ps); i++ {
		// If current node has catchAll, record last
		// seen catchAll
		if ca, exists := node.children[catchAll]; exists {
			lastCatchAll = ca
		}

		segment := ps[i]
		child, exists := node.children[segment]
		if !exists {
			// Child path doesn't exist, check for wildcard
			child, exists = node.children[wcStr]
			if !exists {
				// No wildcard, check for catch call
				if lastCatchAll != nil {
					node = lastCatchAll
				}
				break
			}
			if child.regex != nil && !child.regex.MatchString(segment) {
				// No matching child, and path doesn't match wildcard
				break
			}
			results.addPair(child.wildcard, segment)
		}
		node = child
	}
	for node.data == nil && node.parent != nil {
		// landed on a nil-node, rewind up the tree
		// until we hit a non-nil node or the root
		node = node.parent
	}

	results.data = node.data
	return results
}

// Match returns the object registered at path or an error if none exist.
// Wildcard segments are observed. ErrNotFound is returned if no matching path
// exists and a trailing slash redirect (tsr) isn't possible. ErrRedirect is returned
// if no matching path exists but a tsr is possible.
func (m *DefaultMatcher) Match(path string) (Results, error) {
	if m.root == nil {
		return nil, ErrNotFound
	}

	ps := m.splitPath(path)
	node := m.root
	results := newResults(len(ps))
	var lastCatchAll *matcherNode

	i := 0
	for ; i < len(ps); i++ {
		// If node contains catchAll, record last
		// seen catchAll
		if ca, exists := node.children[catchAll]; exists {
			lastCatchAll = ca
		}

		child, exists := node.children[ps[i]]
		if !exists {
			// Check case where segment is trailing slash and
			// data may be one level up
			if i > 0 && i == len(ps)-1 && ps[i] == empty {
				if checkParentForMatch(node, ps[i-1]) {
					return nil, ErrRedirectSlash
				}
				// No node found for segment or wildcard
				// Send not found signal
				return nil, ErrNotFound
			}

			// Check for wildcard
			child, exists = node.children[wcStr]
			if !exists {
				// No wildcard, check for catchAll
				if lastCatchAll != nil {
					node = lastCatchAll
					break
				}
				return nil, ErrNotFound
			}
			// Found wildcard, check for regexp constraint
			if child.regex != nil && !child.regex.MatchString(ps[i]) {
				return nil, ErrNotFound
			}
			results.addPair(child.wildcard, ps[i])
		}
		node = child
	}

	if node.data == nil {
		// Check case where segment is trailing slash and data
		// may be one level up. We check again here because the
		// trailing slash node might exist so we land on it but be nil due to a delete
		if i > 0 && ps[i-1] == empty && checkParentForMatch(node.parent, ps[i-2]) {
			return nil, ErrRedirectSlash
		}
		// Check case where data might be in trailing slash node
		if child, exists := node.children[empty]; exists {
			if child.data != nil {
				return nil, ErrRedirectSlash
			}
		}
		return nil, ErrNotFound
	}

	results.data = node.data
	return results, nil
}

func (m *DefaultMatcher) MaxParams() int {
	return m.maxParams
}

// PrefixMatch returns all objects strictly matching the prefix. Wildcard segments
// are not observed and thus, to match them, one must use '*' path segments.
func (m *DefaultMatcher) PrefixMatch(prefix string) []interface{} {
	if m.root == nil {
		return make([]interface{}, 0)
	}

	ps := m.splitPath(prefix)
	var results []interface{}

	// Traverse down tree until we run out of path segments
	// or we get to the first node who has no children matching
	// the next segment (the root of the subtree)
	node := m.root
	for i := 0; i < len(ps); i++ {
		segment := ps[i]
		child, exists := node.children[segment]
		if !exists {
			break
		}
		node = child
	}

	// If we are not the node we started at (which means there's no prefix match),
	// we do a BFS of the subtree rooted at node adding the data for
	if node != m.root {
		if node.data != nil {
			results = append(results, node.data)
		}

		var queue []*matcherNode
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

// Splits the path along '/'s, account for leading slashes.
func (m *DefaultMatcher) splitPath(path string) []string {
	pathSplit := strings.Split(path, "/")
	if len(pathSplit) > 0 && len(pathSplit[0]) == 0 {
		pathSplit = pathSplit[1:]
	}
	return pathSplit
}

// Check the current node for a TSR to a non-slash node
func checkParentForMatch(n *matcherNode, path string) bool {
	if n == nil || n.parent == nil {
		return false
	}
	if n, ok := n.parent.children[path]; ok {
		if n.data != nil {
			return true
		}
	}
	if n, ok := n.parent.children[wcStr]; ok {
		if n.data != nil {
			return true
		}
	}
	return false
}
