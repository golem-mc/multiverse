package inventories

import (
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// Sharable is one kind of player data a group can share: the inventory,
// health, experience, ... MV: Sharable, DefaultSharable
type Sharable struct {
	names    []string
	optional bool
	// stat sharables are saved as strings in the profile's "stats" object,
	// the others under their own file tag.
	stat    bool
	fileTag string
	// read stores the player's value in d. MV: SharableHandler.updateProfile
	read func(m *Module, s *Sharable, p plugin.Player, d *ProfileData)
	// apply sets the player from d, defaulting what d lacks, and reports
	// whether d had a value. MV: SharableHandler.updatePlayer
	apply func(m *Module, s *Sharable, p plugin.Player, d *ProfileData) bool
	// encode and decode convert a value to and from its JSON form.
	encode func(m *Module, v any) any
	decode func(m *Module, raw any) (any, bool)
}

// Name answers the sharable's name, as groups.yml and the commands write it.
func (s *Sharable) Name() string { return s.names[0] }

// Names answers the name and the other names it is looked up by.
func (s *Sharable) Names() []string { return slices.Clone(s.names) }

// Optional reports whether the sharable is used only when listed in
// active-optional-shares.
func (s *Sharable) Optional() bool { return s.optional }

func (s *Sharable) String() string { return s.Name() }

// Shares is an ordered set of sharables. MV: Shares, Sharables
type Shares struct{ list []*Sharable }

// SharesOf answers a set of the sharables given.
func SharesOf(ss ...*Sharable) *Shares {
	out := &Shares{}
	for _, s := range ss {
		out.Add(s)
	}
	return out
}

// Has reports whether s is in the set. MV: isSharing
func (sh *Shares) Has(s *Sharable) bool { return slices.Contains(sh.list, s) }

// HasAll reports whether every sharable of o is in the set.
func (sh *Shares) HasAll(o *Shares) bool {
	for _, s := range o.list {
		if !sh.Has(s) {
			return false
		}
	}
	return true
}

// Add adds s, at the end when new.
func (sh *Shares) Add(s *Sharable) {
	if !sh.Has(s) {
		sh.list = append(sh.list, s)
	}
}

// AddAll adds every sharable of o.
func (sh *Shares) AddAll(o *Shares) {
	for _, s := range o.list {
		sh.Add(s)
	}
}

// Remove takes s out.
func (sh *Shares) Remove(s *Sharable) {
	sh.list = slices.DeleteFunc(sh.list, func(x *Sharable) bool { return x == s })
}

// RemoveAll takes every sharable of o out.
func (sh *Shares) RemoveAll(o *Shares) {
	sh.list = slices.DeleteFunc(sh.list, o.Has)
}

// Clone answers a copy.
func (sh *Shares) Clone() *Shares { return &Shares{list: slices.Clone(sh.list)} }

// List answers the sharables in order.
func (sh *Shares) List() []*Sharable { return slices.Clone(sh.list) }

// Len answers how many sharables the set holds.
func (sh *Shares) Len() int { return len(sh.list) }

// Empty reports whether the set holds none.
func (sh *Shares) Empty() bool { return len(sh.list) == 0 }

// Compare answers the sharables of o that are in the set too.
func (sh *Shares) Compare(o *Shares) *Shares {
	out := &Shares{}
	for _, s := range o.list {
		if sh.Has(s) {
			out.Add(s)
		}
	}
	return out
}

// Strings answers the names, or just "all" for a set of every sharable.
// MV: Sharables.toStringList
func (sh *Shares) Strings() []string {
	if sh.HasAll(allShares) {
		return []string{"all"}
	}
	out := make([]string, len(sh.list))
	for i, s := range sh.list {
		out[i] = s.Name()
	}
	return out
}

func (sh *Shares) String() string {
	names := make([]string, len(sh.list))
	for i, s := range sh.list {
		names[i] = s.Name()
	}
	return strings.Join(names, ", ")
}

// sharesNamed holds the sharables and groups of sharables by every name
// they go by, lowercase. MV: Sharables.LOOKUP_MAP
var sharesNamed = map[string]*Shares{}

// LookupShares answers the sharable or group of sharables called name.
// MV: Sharables.lookup
func LookupShares(name string) (*Shares, bool) {
	sh, ok := sharesNamed[strings.ToLower(name)]
	if !ok {
		return nil, false
	}
	return sh.Clone(), true
}

// ShareNames answers every name a share or group of shares goes by.
// MV: Sharables.getShareNames
func ShareNames() []string {
	out := make([]string, 0, len(sharesNamed))
	for name := range sharesNamed {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func isAll(name string) bool {
	return name == "*" || strings.EqualFold(name, "all") || strings.EqualFold(name, "everything")
}

// SharesFromList reads the names that are not negated in list.
// MV: Sharables.fromList
func SharesFromList(list []string) *Shares {
	out := &Shares{}
	for _, name := range list {
		if sh, ok := sharesNamed[strings.ToLower(name)]; ok {
			out.AddAll(sh)
		} else if isAll(name) {
			return allShares.Clone()
		}
	}
	return out
}

// NegativeSharesFromList reads the names negated with "-" in list.
// MV: Sharables.negativeFromList
func NegativeSharesFromList(list []string) *Shares {
	out := &Shares{}
	for _, name := range list {
		name, ok := strings.CutPrefix(name, "-")
		if !ok || name == "" {
			continue
		}
		if sh, ok := sharesNamed[strings.ToLower(name)]; ok {
			out.AddAll(sh)
		} else if isAll(name) {
			return allShares.Clone()
		}
	}
	return out
}

// sharesByTag finds a sharable by the key it is saved under.
func sharableByTag(stat bool, tag string) (*Sharable, bool) {
	for _, s := range allShares.list {
		if s.fileTag == tag && s.stat == stat {
			return s, true
		}
	}
	return nil, false
}

// register adds a sharable to the registry. MV: Sharables.register
func register(s *Sharable) *Sharable {
	allShares.Add(s)
	if s.optional {
		optionalShares.Add(s)
	} else {
		standardShares.Add(s)
	}
	for _, name := range s.names {
		key := strings.ToLower(name)
		if sharesNamed[key] == nil {
			sharesNamed[key] = &Shares{}
		}
		sharesNamed[key].Add(s)
	}
	return s
}

// group names a fixed group of sharables. MV: SharableGroup
func group(shares *Shares, names ...string) *Shares {
	for _, name := range names {
		sharesNamed[name] = shares
	}
	return shares
}

var (
	allShares      = &Shares{}
	standardShares = &Shares{}
	optionalShares = &Shares{}
)

// AllShares, StandardShares and OptionalShares answer copies of every
// sharable, the ones always used and the ones used only when active.
func AllShares() *Shares      { return allShares.Clone() }
func StandardShares() *Shares { return standardShares.Clone() }
func OptionalShares() *Shares { return optionalShares.Clone() }
