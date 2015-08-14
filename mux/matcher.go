package mux

import (
	"errors"
	"regexp"
	"strings"
)

// ---------- Mux Errors ----------
// --------------------------------

// ErrNotFound gets returned by Matcher if a path could not be matched.
var ErrNotFound = errors.New("mux: handler not found")

// ErrNotImplemented gets returned by Matcher if a path could be matched
// but the method could not be found.
var ErrNotImplemented = errors.New("mux: handler not implemented")

// ErrRedirectSlash gets returned by Matcher if a path could not be matched
// but a path with (without) a slash exists.
var ErrRedirectSlash = errors.New("mux: redirect trailing slash")

// ---------- Constants ----------
// -------------------------------

const catchAll string = "^"
const empty string = ""

// ---------- Param ----------
// ---------------------------

// Param represents a Key-Value HTTP parameter pair
type Param struct {
	Key   string
	Value string
}

// ---------- Results ----------
// -----------------------------

// Results is an interface for returning results from the matcher
type Results interface {
	// Returns the resulting data from the path match
	Data() interface{}

	// Returns all parameter key-value pairs as a slice
	Params() []Param
}

// ---------- Matcher ----------
// -----------------------------

// Matcher is an interface for a matcher that matches paths to objects.
// { } denotes a wildcard segment. ^ denotes a catch-all segment.
type Matcher interface {
	// Add adds an object to the matcher registered to the path.
	Add(path string, object interface{})

	// Apply applies a function f to all non-nil object stored
	// in Matcher
	Apply(f func(object interface{}))

	// Match returns the the data and values associated with
	// the path or an error if the path isn't cannot be found.
	// Should return ErrRedirectSlash if a trailing slash redirect
	// is possible.
	Match(path string) (Results, error)

	// MatchNoRegex performs the same as Match except without
	// doing regex checking for wildcard parameters.
	MatchNoRegex(path string) (Results, error)

	// Returns the maximum possible number of wildcard parameters
	MaxParams() int
}

// ---------- matcherResults -----------
// -------------------------------------

// matcherResults is a simple and efficient
// implementation of the Results interface
type matcherResults struct {
	data  interface{}
	pairs []Param
}

func newResults(maxParams int) *matcherResults {
	return &matcherResults{
		pairs: make([]Param, 0, maxParams),
	}
}

func (mr *matcherResults) addPair(key, value string) {
	pair := Param{key, value}
	mr.pairs = append(mr.pairs, pair)
}

func (mr *matcherResults) Data() interface{} {
	return mr.data
}

func (mr *matcherResults) Params() []Param {
	return mr.pairs
}

// ---------- pathIndexer ----------
// ---------------------------------

// pathIndexer returns the start and end indexes
// of path segments separated by /'s for efficient
// inline path parsing.
type pathIndexer struct {
	path   string
	sBegin int
	ts     bool
}

// Returns true if the indexer lies on the trailing slash
func (p *pathIndexer) atTrailingSlash() bool {
	if p.sBegin != len(p.path) || len(p.path) == 0 {
		return false
	}
	if p.path[p.sBegin-1] != '/' {
		return false
	}
	p.ts = true
	return p.ts
}

// Returns true if the indexer was once lying on the trailing slash.
// This is necessary because calling next() on the trailing slash
// will advance the indexer such that atTrailingSlash() returns false
// but we need to know if there was a trailing slash for redirects.
func (p *pathIndexer) seenTrailingSlash() bool {
	return p.ts
}

// Returns true if there is more of the path to index or we are at
// the trailing slash
func (p *pathIndexer) hasNext() bool {
	return p.sBegin < len(p.path) || p.atTrailingSlash()
}

// Returns the next path segment delimited by slashes
// or an empty string if we lay on the trailing slash.
func (p *pathIndexer) next() string {
	i := p.sBegin
	if p.atTrailingSlash() {
		p.sBegin++
		return p.path[i:]
	}
	if i == 0 && p.path[i] == '/' {
		i++
	}

	j := i
	for j < len(p.path) && p.path[j] != '/' {
		j++
	}
	p.sBegin = j + 1
	return p.path[i:j]
}

// --------- matcherNode ----------
// --------------------------------

// matcherNode is the k-ary node used in the
// DefaultMatcher's tree
type matcherNode struct {
	data      interface{}
	parent    *matcherNode
	children  map[string]*matcherNode
	wildChild *matcherNode
	catchAll  *matcherNode

	wildcard string
	regex    *regexp.Regexp
}

func newMatcherNode() *matcherNode {
	return &matcherNode{
		children: make(map[string]*matcherNode),
	}
}

// ---------- DefaultMatcher ----------
// ------------------------------------

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

	pi := pathIndexer{path: path}
	node := m.root
	nparams := 0

	for pi.hasNext() {
		// Get next path segment
		s := pi.next()

		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			// Path segment is wildcard
			child := node.wildChild
			if child == nil {
				// If no wildcard node, create new one
				child = newMatcherNode()
				child.parent = node
				node.wildChild = child
			}

			wc := strings.TrimPrefix(strings.TrimSuffix(s, "}"), "{")
			wc = strings.TrimSpace(wc)
			if strings.Contains(wc, ":") {
				// Path segment contains regexp
				// Parse out and save regexp
				wcSplit := strings.Split(wc, ":")
				wc = strings.TrimSpace(wcSplit[0])
				regex := strings.TrimSpace(wcSplit[1])

				var err error
				child.regex, err = regexp.Compile(regex)
				if err != nil {
					panic("Could not compile: " + err.Error())
				}
			}
			child.wildcard = wc
			node = child
			nparams++
		} else if s == catchAll {
			// Path segment is catch all
			child := node.catchAll
			if child == nil {
				child = newMatcherNode()
				child.parent = node
				node.catchAll = child
			}
			child.data = object
			return
		} else {
			// Get or add node for this segment and move on
			child, ok := node.children[s]
			if !ok {
				child = newMatcherNode()
				child.parent = node
				node.children[s] = child
			}
			node = child
		}
	}
	if nparams > m.maxParams {
		m.maxParams = nparams
	}
	node.data = object
}

// Apply does a BFS traversal of the matcher tree and applies
// function f to all non-nil objects stored in the tree
func (m *DefaultMatcher) Apply(f func(object interface{})) {
	queue := make([]*matcherNode, 1)
	queue[0] = m.root

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, child := range node.children {
			queue = append(queue, child)
		}

		if node.data != nil {
			f(node.data)
		}
	}
}

// Match returns the object registered at path or an error if none exist.
// Wildcard segments are observed. ErrNotFound is returned if no matching path
// exists and a trailing slash redirect (tsr) isn't possible. ErrRedirect is returned
// if no matching path exists but a tsr is possible.
func (m *DefaultMatcher) Match(path string) (Results, error) {
	return m.match(path, true)
}

// MatchNoRegex performs in the same manner as Match except that it doesn't
// check regex restrictions on wildcard parameters.
func (m *DefaultMatcher) MatchNoRegex(path string) (Results, error) {
	return m.match(path, false)
}

// MaxParams returns the maximum possible number of
// parameters in the Matcher based on the added paths.
func (m *DefaultMatcher) MaxParams() int {
	return m.maxParams
}

// Private matching function that contains all the matching logic
func (m *DefaultMatcher) match(path string, regex bool) (Results, error) {
	if m.root == nil {
		return nil, ErrNotFound
	}

	pi := pathIndexer{path: path}
	node := m.root
	results := newResults(m.maxParams)

	for pi.hasNext() {
		s := pi.next()
		child, ok := node.children[s]
		if !ok {
			// No child found, check for case where we are
			// at trailing slash and a redirect might be in order
			if pi.seenTrailingSlash() {
				if node.data != nil {
					return nil, ErrRedirectSlash
				}
				if node.parent.wildChild != nil && node.parent.wildChild.data != nil {
					return nil, ErrRedirectSlash
				}
				return nil, ErrNotFound
			}

			// Not at trailing slash so we check for a possible
			// wildcard segment
			if child = node.wildChild; child == nil {
				// No wildcard so our last resort is a catch all segment
				if node.catchAll != nil {
					node = node.catchAll
					break
				}
				return nil, ErrNotFound
			}

			// Found wildcard, check the regex constraint if necessary
			if regex && child.regex != nil && !child.regex.MatchString(s) {
				return nil, ErrNotFound
			}
			results.addPair(child.wildcard, s)
		}
		node = child
	}
	if node.data == nil {
		// If we are at a node whose data is nil, it is most likely the
		// case that the data actually lies on a trailing slash node
		if child, ok := node.children[empty]; ok && child.data != nil {
			return nil, ErrRedirectSlash
		}
		return nil, ErrNotFound
	}

	results.data = node.data
	return results, nil
}
