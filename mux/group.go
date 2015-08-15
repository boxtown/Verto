package mux

import (
	"net/http"
)

type Groupable interface {
	Join(group *Group)
	Use(handler PluginHandler) Groupable
	UseHandler(handler http.Handler) Groupable
}

type Group struct {
	parent   *Group
	members  []Groupable
	chain    *plugins
	compiled *plugins
}

func NewGroup() *Group {
	return &Group{
		members: make([]Groupable, 0),
		chain:   newPlugins(),
	}
}

// Add adds a groupable to the members in g
func (g *Group) Add(groupable Groupable) *Group {
	groupable.Join(g)
	g.members = append(g.members, groupable)
	return g
}

// In performs a DFS on members of group and their
// subgroups to determine if g is either in group
// or one of the subgroups. Returns true if g is
// is found and false otherwise.
func (g *Group) In(group *Group) bool {
	// s is stack of edges to travel
	// seen keeps track of discovered edges
	seen := make(map[Groupable]bool)
	s := make([]Groupable, len(g.members))
	copy(s, g.members)

	for len(s) > 0 {
		// pop s
		v := s[0]
		s = s[1:]

		// If popped edge is not discovered,
		// mark it as discovered. If edge is
		// Group, check members for g. If found,
		// then g in group. Otherwise push g's
		// children and go again.
		if _, ok := seen[v]; !ok {
			seen[v] = true
			if g, ok := v.(*Group); ok {
				for _, m := range g.members {
					if m == g {
						return true
					}
				}
				s = append(s, g.members...)
			}
		}
	}
	return false
}

// Join sets the parent of g as the Group
// g just joined. If g is a member of another group,
// g leaves that group for the new group
func (g *Group) Join(group *Group) {
	if g == group {
		return
	}
	if g.parent != nil {
		g.Leave(g.parent)
	}
	g.parent = group
}

// Leave deletes g from the group if g is a
// member of that group
func (g *Group) Leave(group *Group) {
	for i := 0; i < len(group.members); i++ {
		if group.members[i] == g {
			// GC-safe delete of g
			// from group.members
			copy(group.members[i:], group.members[i+1:])
			group.members[len(group.members)-1] = nil
			group.members = group.members[:len(group.members)-1]
			break
		}
	}
	g.parent = nil
}

// Use adds a PluginHandler onto the end of the chain of plugins
// for a Group.
func (g *Group) Use(handler PluginHandler) Groupable {
	g.chain.use(handler)
	g.compile()
	for _, m := range g.members {
		m.Use(handler)
	}
	return g
}

// UseHandler wraps the handler as a PluginHandler and adds it onto the end
// of the plugin chain.
func (g *Group) UseHandler(handler http.Handler) Groupable {
	pluginHandler := PluginFunc(
		func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
			handler.ServeHTTP(w, r)
			next(w, r)
		})

	return g.Use(pluginHandler)
}

// Compiles and stores the chain of plugins
// starting with the root group working its
// way down to g. This works because any group
// may be a sub-group of only one other group
func (g *Group) compile() {
	var p *plugins
	if g.parent != nil {
		p = g.parent.compiled
		p.link(g.chain.deepCopy())
	} else {
		p = g.chain.deepCopy()
	}
	g.compiled = p
}
