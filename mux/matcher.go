package mux

import (
	"errors"
	"regexp"
	"strings"
)

// ---------- Mux Errors ----------
// --------------------------------

// ErrNotFound gets returned if a path could not be matched.
var ErrNotFound = errors.New("mux: handler not found")

// ErrNotImplemented gets returned if a path could be matched
// but the method could not be found.
var ErrNotImplemented = errors.New("mux: handler not implemented")

// ErrRedirectSlash gets returned if a path could not be matched
// but a path with (without) a slash exists.
var ErrRedirectSlash = errors.New("mux: redirect trailing slash")

// ---------- Constants ----------
// -------------------------------

const catchAll string = "^"
const empty string = ""

// ---------- Param ----------
// ---------------------------

// param represents a Key-Value HTTP parameter pair
type param struct {
	key   string
	value string
}

// ---------- Results ----------
// -----------------------------

// results is an interface for returning results from the matcher
type results interface {
	// Returns the resulting data from the path match
	data() compilable

	// Returns all parameter key-value pairs as a slice
	params() []param
}

// ---------- matcherResults -----------
// -------------------------------------

// matcherResults is a simple and efficient
// implementation of the Results interface
type matcherResults struct {
	c compilable
	p []param
}

func newResults(maxParams int) *matcherResults {
	return &matcherResults{
		p: make([]param, 0, maxParams),
	}
}

func (mr *matcherResults) addPair(key, value string) {
	pair := param{key, value}
	mr.p = append(mr.p, pair)
}

func (mr *matcherResults) data() compilable {
	return mr.c
}

func (mr *matcherResults) params() []param {
	return mr.p
}

// ---------- pathIterator ----------
// ---------------------------------

// pathIterator returns the start and end indexes
// of path segments separated by /'s for efficient
// inline path parsing.
type pathIterator struct {
	path   string
	sBegin int
	ts     bool
}

// Returns true if the indexer lies on the trailing slash
func (p *pathIterator) atTrailingSlash() bool {
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
func (p *pathIterator) seenTrailingSlash() bool {
	return p.ts
}

// Returns true if there is more of the path to index or we are at
// the trailing slash
func (p *pathIterator) hasNext() bool {
	return p.sBegin < len(p.path) || p.atTrailingSlash()
}

// Returns the next path segment delimited by slashes
// or an empty string if we lay on the trailing slash.
func (p *pathIterator) next() string {
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
	data      compilable
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

// Private function that adds object as data at path and returns
// number of encountered path parameters
func (n *matcherNode) add(path string, c compilable) int {
	pi := pathIterator{path: path}
	nparams := 0

	for pi.hasNext() {
		// Get next path segment
		s := pi.next()
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			// Path segment is wildcard
			child := n.wildChild
			if child == nil {
				// If no wildcard node, create new one
				child = newMatcherNode()
				child.parent = n
				n.wildChild = child
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
			n = child
			nparams++
		} else if s == catchAll {
			// Path segment is catch all
			child := n.catchAll
			if child == nil {
				child = newMatcherNode()
				child.parent = n
				n.catchAll = child
			}
			child.data = c
			return nparams
		} else {
			// Get or add node for this segment and move on
			child, ok := n.children[s]
			if !ok {
				child = newMatcherNode()
				child.parent = n
				n.children[s] = child
			}
			n = child
		}
	}
	n.data = c
	return nparams
}

// Private apply function that applys f to the objects
// at n and all its subpaths in BFS order
func (n *matcherNode) apply(f func(c compilable)) {
	queue := make([]*matcherNode, 1)
	queue[0] = n

	for len(queue) > 0 {
		n = queue[0]
		queue = queue[1:]
		for _, child := range n.children {
			queue = append(queue, child)
		}

		if n.data != nil {
			f(n.data)
		}
	}
}

// Applys f to the subtree rooted at path. Automatically stops
// traversal at catch-all. Wildcards must be explicitly matched.
// If the path is not found, the function returns without applying
// f.
func (n *matcherNode) applyAt(path string, f func(c compilable)) {
	pi := pathIterator{path: path}
	for pi.hasNext() {
		s := pi.next()
		child, ok := n.children[s]
		if !ok {
			if s == catchAll {
				n = n.catchAll
				break
			} else if s[0] == '{' && s[len(s)-1] == '}' {
				child = n.wildChild
				continue
			}
			return
		}
		n = child
	}
	n.apply(f)
}

// Private drop function that drops the subtree
// rooted at path. Automatically stops parsing on a
// catch-all. Wildcards must be matched explicitly with
// starting { and ending }. Dropped subtrees are completely
// deleted.
func (n *matcherNode) drop(path string) {
	pi := pathIterator{path: path}
	var s string

	for pi.hasNext() {
		s = pi.next()
		child, ok := n.children[s]
		if !ok {
			if s == catchAll {
				n = n.catchAll
				break
			} else if s[0] == '{' && s[len(s)-1] == '}' {
				child = n.wildChild
				continue
			}
			return
		}
		n = child
	}
	delete(n.parent.children, s)
}

// Private matching function that contains all the matching logic
func (n *matcherNode) match(path string, explicit bool, maxParams int) (results, error) {
	pi := pathIterator{path: path}
	results := newResults(maxParams)
	var mrg compilable

	for pi.hasNext() {
		s := pi.next()
		child, ok := n.children[s]
		if !ok {
			// No child found, check for case where we are
			// at trailing slash and a redirect might be in order
			if pi.seenTrailingSlash() {
				if n.data != nil {
					return nil, ErrRedirectSlash
				}
				if n.parent.wildChild != nil && n.parent.wildChild.data != nil {
					return nil, ErrRedirectSlash
				}
				return nil, ErrNotFound
			}

			var notWild = len(s) == 0 || s[0] != '{' || s[len(s)-1] != '}'
			child = n.wildChild

			// If segment is not wild and we want explicit match
			// or wild child doesn't exist, check catch all and
			// most recent group as last ditch effort
			if (explicit && notWild) || child == nil {
				if n.catchAll != nil {
					n = n.catchAll
					break
				}
				if mrg != nil {
					results.c = mrg
					return results, nil
				}
				return nil, ErrNotFound
			}

			// Found wildcard, check the regex constraint if necessary
			if !explicit && child.regex != nil && !child.regex.MatchString(s) {
				return nil, ErrNotFound
			}
			results.addPair(child.wildcard, s)
		}
		if child.data != nil && child.data.cType() == GROUP {
			mrg = child.data
		}
		n = child
	}
	if n.data == nil {
		// If we are at a node whose data is nil, it is most likely the
		// case that the data actually lies on a trailing slash node
		if child, ok := n.children[empty]; ok && child.data != nil {
			return nil, ErrRedirectSlash
		}
		return nil, ErrNotFound
	}

	results.c = n.data
	return results, nil
}

// ---------- DefaultMatcher ----------
// ------------------------------------

// defaultMatcher is the default implementation
// of the matcher interface.
type matcher struct {
	root *matcherNode
	mp   int
}

// Add registers an object with a specific path. Wildcard path
// segments are denoted by {}'s. The string within the brackets is
// used as the key for key-value parameter pairs when matching a path.
// Regex can be defined inside wildcard path segments by appending a colon
// and a regex after the inner string. Catch-all paths are denoted with
// a '^'. Any path segments after a catch-all symbol are ignored as it
// does not make any sense to have child paths of a catch-all path.
func (m *matcher) add(path string, c compilable) {
	if m.root == nil {
		m.root = newMatcherNode()
	}
	nparams := m.root.add(path, c)
	if nparams > m.mp {
		m.mp = nparams
	}
}

// Apply does a BFS traversal of the matcher tree and applies
// function f to all non-nil objects stored in the tree
func (m *matcher) apply(f func(c compilable)) {
	if m.root == nil {
		return
	}
	m.root.apply(f)
}

// Apply traverses the matcher tree until path is matched and then
// applies f to all subpaths rooted at path including path. Traversal
// automatically stops at a catch-all. Wildcards must be explicitly matched.
func (m *matcher) applyAt(path string, f func(c compilable)) {
	if m.root == nil {
		return
	}
	m.root.applyAt(path, f)
}

// Drop drops the subtree rooted at path
func (m *matcher) drop(path string) {
	if m.root == nil {
		return
	}
	m.root.drop(path)
}

// Match returns the object registered at path or an error if none exist.
// Wildcard segments are observed. ErrNotFound is returned if no matching path
// exists and a trailing slash redirect (tsr) isn't possible. ErrRedirect is returned
// if no matching path exists but a tsr is possible.
func (m *matcher) match(path string) (results, error) {
	if m.root == nil {
		return nil, ErrNotFound
	}
	return m.root.match(path, false, m.mp)
}

// MatchNoRegex performs in the same manner as Match except that it doesn't
// check regex restrictions on wildcard parameters.
func (m *matcher) matchExplicit(path string) (results, error) {
	if m.root == nil {
		return nil, ErrNotFound
	}
	return m.root.match(path, true, m.mp)
}

// MaxParams returns the maximum possible number of
// parameters in the Matcher based on the added paths.
func (m *matcher) maxParams() int {
	return m.mp
}
