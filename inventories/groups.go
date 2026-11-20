package inventories

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"go.yaml.in/yaml/v4"
)

// defaultGroupName is the group ungrouped worlds fall into with
// default-ungrouped-worlds.
const defaultGroupName = "default"

// WorldGroup is a group of worlds that share some of their players' data.
// MV: WorldGroup
type WorldGroup struct {
	m    *Module
	name string
	// configWorlds are the worlds as groups.yml lists them, lowercase: names,
	// wildcards ("world_*") or regexes ("r=...").
	configWorlds []string
	shares       *Shares
	disabled     *Shares
	spawnWorld   string
	// spawnPriority is the respawn listener priority at which the group's
	// spawn is used, as Bukkit's EventPriority names it.
	spawnPriority string

	matcher          matcherGroup
	applicable       map[string]bool
	applicableShares *Shares
}

// Name answers the group's name.
func (g *WorldGroup) Name() string { return g.name }

// ConfigWorlds answers the worlds as groups.yml lists them.
func (g *WorldGroup) ConfigWorlds() []string { return slices.Clone(g.configWorlds) }

// ApplicableWorlds answers the loaded worlds the group holds, lowercase.
func (g *WorldGroup) ApplicableWorlds() []string {
	out := make([]string, 0, len(g.applicable))
	for w := range g.applicable {
		out = append(out, w)
	}
	slices.Sort(out)
	return out
}

// ContainsWorld reports whether the group holds a loaded world.
func (g *WorldGroup) ContainsWorld(name string) bool { return g.applicable[strings.ToLower(name)] }

// Shares answers the shares groups.yml lists; DisabledShares the ones it
// disables; ApplicableShares those in use: the shares, less the disabled
// ones and the optional ones not active.
func (g *WorldGroup) Shares() *Shares           { return g.shares }
func (g *WorldGroup) DisabledShares() *Shares   { return g.disabled }
func (g *WorldGroup) ApplicableShares() *Shares { return g.applicableShares }

// IsSharing reports whether the group shares s.
func (g *WorldGroup) IsSharing(s *Sharable) bool { return g.applicableShares.Has(s) }

// SpawnWorld answers the world players of the group respawn in, "" for none.
func (g *WorldGroup) SpawnWorld() string { return g.spawnWorld }

// AddWorld, AddWorlds and RemoveWorlds change the group's worlds; save
// writes groups.yml after. MV: WorldGroup.addWorld, removeWorlds
func (g *WorldGroup) AddWorld(name string, save bool) { g.AddWorlds([]string{name}, save) }

func (g *WorldGroup) AddWorlds(names []string, save bool) {
	for _, n := range names {
		n = strings.ToLower(n)
		if !slices.Contains(g.configWorlds, n) {
			g.configWorlds = append(g.configWorlds, n)
		}
	}
	if save {
		g.m.Groups.Update(g)
	}
}

func (g *WorldGroup) RemoveWorlds(names []string, save bool) bool {
	before := len(g.configWorlds)
	g.configWorlds = slices.DeleteFunc(g.configWorlds, func(w string) bool {
		return slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(n, w) })
	})
	changed := len(g.configWorlds) != before
	if changed && save {
		g.m.Groups.Update(g)
	}
	return changed
}

// RemoveAllWorlds empties the group's world list.
func (g *WorldGroup) RemoveAllWorlds(save bool) {
	g.configWorlds = nil
	if save {
		g.m.Groups.Update(g)
	}
}

// IsDefault reports whether this is the default group.
func (g *WorldGroup) IsDefault() bool { return g.name == defaultGroupName }

// recalculateWorlds finds the loaded worlds the group's patterns match.
// MV: WorldGroup.recalculateApplicableWorlds
func (g *WorldGroup) recalculateWorlds() {
	g.matcher = newMatcherGroup(g.configWorlds, g.m.Log)
	g.applicable = map[string]bool{}
	for _, w := range g.m.Server.Worlds() {
		if name := strings.ToLower(g.m.worldName(w)); g.matcher.matches(name) {
			g.applicable[name] = true
		}
	}
}

// recalculateShares works out the shares in use. MV: recalculateApplicableShares
func (g *WorldGroup) recalculateShares() {
	g.applicableShares = g.shares.Clone()
	g.applicableShares.RemoveAll(g.disabled)
	inactive := optionalShares.Clone()
	inactive.RemoveAll(g.m.activeOptionalShares())
	g.applicableShares.RemoveAll(inactive)
}

func (g *WorldGroup) String() string {
	s := g.name + ": {Worlds: [" + strings.Join(g.configWorlds, ", ") + "], Shares: [" + g.shares.String() + "]"
	if g.spawnWorld != "" {
		s += ", Spawn World: " + g.spawnWorld + ", Spawn Priority: " + g.spawnPriority
	}
	return s + "}"
}

// matcherGroup matches world names against exact names, wildcards and
// "r=" regexes. MV: MatcherGroup (Multiverse-Core)
type matcherGroup struct {
	exact    map[string]bool
	patterns []*regexp.Regexp
}

func newMatcherGroup(list []string, log *plugin.Logger) matcherGroup {
	mg := matcherGroup{exact: map[string]bool{}}
	for _, s := range list {
		switch {
		case s == "":
		case strings.HasPrefix(s, "r="):
			re, err := regexp.Compile("^(?:" + s[2:] + ")$")
			if err != nil {
				log.Warn(fmt.Sprintf("Failed to compile regex '%s': %v", s[2:], err))
				continue
			}
			mg.patterns = append(mg.patterns, re)
		case strings.Contains(s, "*"):
			parts := strings.Split(s, "*")
			for i, p := range parts {
				parts[i] = regexp.QuoteMeta(p)
			}
			mg.patterns = append(mg.patterns, regexp.MustCompile("^"+strings.Join(parts, ".*")+"$"))
		default:
			mg.exact[s] = true
		}
	}
	return mg
}

func (mg matcherGroup) matches(s string) bool {
	if mg.exact[s] {
		return true
	}
	for _, re := range mg.patterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// Groups is groups.yml's world groups. MV: YamlWorldGroupManager,
// AbstractWorldGroupManager
type Groups struct {
	m    *Module
	path string
	// list holds the groups in groups.yml's order.
	list []*WorldGroup
}

func newGroups(m *Module) *Groups {
	return &Groups{m: m, path: filepath.Join(m.Server.DataDir(), "groups.yml")}
}

// groupsHeader is groups.yml's comment above its groups.
const groupsHeader = `# Multiverse-Inventories Groups

# To ADD, DELETE, and EDIT groups use the command /mvinv group.
# No support will be given for those who manually edit these groups.
`

// Load reads groups.yml; without one it is made from the groups an old
// config.yml held. MV: YamlWorldGroupManager.load
func (gs *Groups) Load() error {
	gs.list = nil
	data, err := os.ReadFile(gs.path)
	var raw map[string]any
	switch {
	case errors.Is(err, os.ErrNotExist):
		gs.m.Debug(1, "Created groups file.")
		if gs.m.legacyGroups != nil {
			raw = map[string]any{"groups": gs.m.legacyGroups}
			gs.m.Debug(1, "Migrated groups to groups.yml")
		}
	case err != nil:
		return err
	default:
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("groups.yml: %w", err)
		}
	}
	gs.m.legacyGroups = nil
	section, _ := raw["groups"].(map[string]any)
	for _, name := range orderedGroupNames(data, section) {
		g, ok := gs.decode(name, section[name])
		if !ok {
			continue
		}
		gs.list = append(gs.list, g)
	}
	if section == nil {
		gs.m.Log.Info("No world groups have been configured!")
		gs.m.Log.Info("This will cause all worlds configured for Multiverse to have separate player statistics/inventories.")
	}
	return gs.save()
}

// orderedGroupNames answers the group names in the file's order.
func orderedGroupNames(data []byte, section map[string]any) []string {
	var doc yaml.Node
	var out []string
	if yaml.Unmarshal(data, &doc) == nil && len(doc.Content) > 0 {
		root := doc.Content[0]
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value != "groups" {
				continue
			}
			groups := root.Content[i+1]
			for j := 0; j+1 < len(groups.Content); j += 2 {
				out = append(out, groups.Content[j].Value)
			}
		}
	}
	for name := range section {
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

func toStrings(raw any) ([]string, bool) {
	l, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(l))
	for _, e := range l {
		if e != nil {
			out = append(out, fmt.Sprint(e))
		}
	}
	return out, true
}

// decode reads one group. MV: YamlWorldGroupManager.deserializeGroup
func (gs *Groups) decode(name string, raw any) (*WorldGroup, bool) {
	data, ok := raw.(map[string]any)
	if !ok {
		gs.m.Log.Warn("Group: '" + name + "' is not formatted correctly!")
		return nil, false
	}
	g := gs.newGroup(name)
	if worlds, ok := data["worlds"]; ok && worlds != nil {
		list, ok := toStrings(worlds)
		if !ok {
			gs.m.Log.Warn("World list formatted incorrectly for world group: " + name)
		}
		var invalid []string
		for _, w := range list {
			g.AddWorld(w, false)
			if !gs.m.validWorldName(w) {
				invalid = append(invalid, w)
			}
		}
		if len(invalid) > 0 {
			gs.m.Log.Warn(fmt.Sprintf("The following worlds for group '%s' does not exist: %s", name, strings.Join(invalid, ", ")))
		}
	}
	readShares := func(key string, into *Shares) {
		v, ok := data[key]
		if !ok {
			return
		}
		list, ok := toStrings(v)
		if !ok {
			gs.m.Log.Warn("Shares formatted incorrectly for group: " + name)
			return
		}
		into.AddAll(SharesFromList(list))
		into.RemoveAll(NegativeSharesFromList(list))
	}
	readShares("shares", g.shares)
	readShares("disabled-shares", g.disabled)
	if spawn, ok := data["spawn"]; ok {
		props, ok := spawn.(map[string]any)
		if !ok {
			gs.m.Log.Warn("Spawn settings for group formatted incorrectly")
		}
		if w, ok := props["world"]; ok {
			g.spawnWorld = strings.ToLower(fmt.Sprint(w))
		}
		if p, ok := props["priority"]; ok {
			if pr := strings.ToUpper(fmt.Sprint(p)); slices.Contains(priorityNames, pr) {
				g.spawnPriority = pr
			}
		}
	}
	g.recalculateWorlds()
	g.recalculateShares()
	return g, true
}

// priorityNames are Bukkit's EventPriority names, lowest first.
var priorityNames = []string{"LOWEST", "LOW", "NORMAL", "HIGH", "HIGHEST", "MONITOR"}

func (gs *Groups) newGroup(name string) *WorldGroup {
	return &WorldGroup{m: gs.m, name: name, shares: &Shares{}, disabled: &Shares{}, spawnPriority: "NORMAL",
		applicable: map[string]bool{}, applicableShares: &Shares{}}
}

// save writes groups.yml. MV: YamlWorldGroupManager.save
func (gs *Groups) save() error {
	groups := &yaml.Node{Kind: yaml.MappingNode}
	for _, g := range gs.list {
		groups.Content = append(groups.Content, scalar(g.name), gs.encode(g))
	}
	root := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{scalar("groups"), groups}}
	var b bytes.Buffer
	b.WriteString(groupsHeader)
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return os.WriteFile(gs.path, b.Bytes(), 0o644)
}

func scalar(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: s} }

func seq(list []string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode}
	for _, s := range list {
		n.Content = append(n.Content, scalar(s))
	}
	return n
}

// encode writes one group. MV: serializeWorldGroupProfile
func (gs *Groups) encode(g *WorldGroup) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{scalar("worlds"), seq(g.configWorlds)}}
	if l := g.shares.Strings(); len(l) > 0 {
		n.Content = append(n.Content, scalar("shares"), seq(l))
	}
	if l := g.disabled.Strings(); len(l) > 0 {
		n.Content = append(n.Content, scalar("disabled-shares"), seq(l))
	}
	if g.spawnWorld != "" {
		n.Content = append(n.Content, scalar("spawn"), &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
			scalar("world"), scalar(g.spawnWorld), scalar("priority"), scalar(g.spawnPriority)}})
	}
	return n
}

// Get answers a group by name, ignoring case. MV: getGroup
func (gs *Groups) Get(name string) (*WorldGroup, bool) {
	for _, g := range gs.list {
		if strings.EqualFold(g.name, name) {
			return g, true
		}
	}
	return nil, false
}

// All answers the groups in order.
func (gs *Groups) All() []*WorldGroup { return slices.Clone(gs.list) }

// ForWorld answers the groups holding a world: with default-ungrouped-worlds,
// the default group for a Multiverse world no group holds.
// MV: getGroupsForWorld
func (gs *Groups) ForWorld(world string) []*WorldGroup {
	var out []*WorldGroup
	for _, g := range gs.list {
		if g.ContainsWorld(world) {
			out = append(out, g)
		}
	}
	if len(out) == 0 && gs.m.Config.Bool("share-handling.default-ungrouped-worlds") {
		if _, ok := gs.m.Core.Worlds.Get(world); ok {
			out = append(out, gs.Default())
		}
	}
	return out
}

// HasConfiguredGroup reports whether a group holds the world.
func (gs *Groups) HasConfiguredGroup(world string) bool {
	return slices.ContainsFunc(gs.list, func(g *WorldGroup) bool { return g.ContainsWorld(world) })
}

// Update stores a new or changed group and writes groups.yml.
// MV: updateGroup
func (gs *Groups) Update(g *WorldGroup) {
	if i := slices.IndexFunc(gs.list, func(x *WorldGroup) bool { return strings.EqualFold(x.name, g.name) }); i >= 0 {
		gs.list[i] = g
	} else {
		gs.list = append(gs.list, g)
	}
	g.recalculateWorlds()
	g.recalculateShares()
	if err := gs.save(); err != nil {
		gs.m.Log.Error("Could not save groups.yml", "error", err)
	}
}

// Remove deletes a group and writes groups.yml. MV: removeGroup
func (gs *Groups) Remove(g *WorldGroup) bool {
	i := slices.Index(gs.list, g)
	if i < 0 {
		return false
	}
	gs.list = slices.Delete(gs.list, i, i+1)
	if err := gs.save(); err != nil {
		gs.m.Log.Error("Could not save groups.yml", "error", err)
	}
	return true
}

// NewGroup answers an empty group, not yet stored; false when the name is
// taken. MV: newEmptyGroup
func (gs *Groups) NewGroup(name string) (*WorldGroup, bool) {
	if _, ok := gs.Get(name); ok {
		return nil, false
	}
	return gs.newGroup(name), true
}

// Default answers the default group, made sharing everything when there
// is none. MV: getDefaultGroup
func (gs *Groups) Default() *WorldGroup {
	if g, ok := gs.Get(defaultGroupName); ok {
		return g
	}
	g := gs.newGroup(defaultGroupName)
	g.shares.AddAll(allShares)
	gs.Update(g)
	return g
}

// createDefault makes the default group of the default world and its
// nether and end. MV: createDefaultGroup
func (gs *Groups) createDefault() {
	if _, ok := gs.Get(defaultGroupName); ok {
		return
	}
	overworld := gs.m.worldName(gs.m.Server.Overworld())
	if w, ok := gs.m.Core.Worlds.Default(); ok {
		overworld = w.Name()
	}
	g := gs.newGroup(defaultGroupName)
	g.shares.AddAll(allShares)
	g.AddWorld(overworld, false)
	for _, suffix := range []string{"_nether", "_the_end"} {
		if _, ok := gs.m.worldNamed(overworld + suffix); ok {
			g.AddWorld(overworld+suffix, false)
		}
	}
	gs.Update(g)
	gs.m.Log.Info("Created a default group for you containing all of your default worlds: " + strings.Join(g.configWorlds, ", "))
}

// RecalculateShares and RecalculateWorlds refresh every group's.
func (gs *Groups) RecalculateShares() {
	for _, g := range gs.list {
		g.recalculateShares()
	}
}

func (gs *Groups) RecalculateWorlds() {
	for _, g := range gs.list {
		g.recalculateWorlds()
	}
}

// worldLoaded and worldUnloaded keep the groups' loaded worlds current.
func (gs *Groups) worldLoaded(name string) {
	name = strings.ToLower(name)
	for _, g := range gs.list {
		if g.matcher.matches(name) {
			g.applicable[name] = true
		}
	}
}

func (gs *Groups) worldUnloaded(name string) {
	for _, g := range gs.list {
		delete(g.applicable, strings.ToLower(name))
	}
}

// Conflict is two groups holding the same world and sharing the same data.
// MV: GroupingConflict
type Conflict struct {
	First, Second *WorldGroup
	Shares        *Shares
}

// Worlds answers the worlds both groups hold.
func (c Conflict) Worlds() []string {
	var out []string
	for _, w := range c.First.ApplicableWorlds() {
		if c.Second.applicable[w] {
			out = append(out, w)
		}
	}
	return out
}

// Conflicts answers the conflicting pairs of groups. MV: checkForConflicts
func (gs *Groups) Conflicts() []Conflict {
	var out []Conflict
	checked := map[[2]*WorldGroup]bool{}
	for _, a := range gs.list {
		for _, w := range a.ApplicableWorlds() {
			for _, b := range gs.ForWorld(w) {
				if a == b || checked[[2]*WorldGroup{a, b}] || checked[[2]*WorldGroup{b, a}] {
					continue
				}
				checked[[2]*WorldGroup{a, b}] = true
				if shared := b.applicableShares.Compare(a.applicableShares); !shared.Empty() {
					out = append(out, Conflict{First: a, Second: b, Shares: shared})
				}
			}
		}
	}
	return out
}
