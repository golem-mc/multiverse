package netherportals

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/golem-mc/multiverse/core"
)

// LinkType is a kind of portal a world links: nether or end.
// MV: WorldLinkType
type LinkType uint8

const (
	Nether LinkType = iota
	End
)

// LinkTypes are the link types in links.yml's order.
var LinkTypes = []LinkType{Nether, End}

// String answers the links.yml key and command name: "nether" or "end".
func (t LinkType) String() string {
	if t == End {
		return "end"
	}
	return "nether"
}

// Message answers the coloured name messages show for {linkType}.
func (t LinkType) Message() core.Message {
	if t == End {
		return core.Lit("&bend")
	}
	return core.Lit("&cnether")
}

// ParseLinkType reads "nether" or "end", ignoring case, failing as ACF's
// enum resolver does.
func ParseLinkType(s string) (LinkType, error) {
	for _, t := range LinkTypes {
		if strings.EqualFold(s, t.String()) {
			return t, nil
		}
	}
	return 0, core.InvalidArg(core.Lit("Please specify one of (nether, end)."))
}

// WorldLink holds the nether and end destinations of one world.
// MV: WorldLink
type WorldLink struct {
	From  string
	links [2]string
}

// To answers the destination for t, false when unset.
func (l *WorldLink) To(t LinkType) (string, bool) { return l.links[t], l.links[t] != "" }

// HasLinks reports whether any destination is set.
func (l *WorldLink) HasLinks() bool { return l.links != [2]string{} }

// Links is links.yml: each world's portal destinations, by world name.
// Changes stay in memory until Save. MV: LinksManager
type Links struct {
	path   string
	links  map[string]*WorldLink
	loaded bool
}

func newLinks(dir string) *Links {
	return &Links{path: filepath.Join(dir, "links.yml"), links: map[string]*WorldLink{}}
}

// Load reads links.yml, creating it when missing; a failed load keeps the
// links read before.
func (ls *Links) Load() error {
	data, err := os.ReadFile(ls.path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(ls.path), 0o755); err != nil {
			return err
		}
		err = os.WriteFile(ls.path, nil, 0o644)
	}
	if err != nil {
		return fmt.Errorf("failed to load links.yml file: %w", err)
	}
	m := map[string]any{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("failed to load links.yml file: %w", err)
	}
	parsed := map[string]*WorldLink{}
	for key, section := range m {
		sec, _ := section.(map[string]any)
		name := decodeKey(key)
		l := &WorldLink{From: name}
		for _, t := range LinkTypes {
			if s, ok := sec[t.String()]; ok && s != nil {
				l.links[t] = fmt.Sprint(s)
			}
		}
		if l.HasLinks() {
			parsed[name] = l
		}
	}
	ls.links = parsed
	ls.loaded = true
	return nil
}

// Loaded reports whether links.yml has been read.
func (ls *Links) Loaded() bool { return ls.loaded }

// Save writes every link to links.yml, worlds in name order.
func (ls *Links) Save() error {
	if !ls.loaded {
		return errors.New("links.yml is not loaded")
	}
	if err := os.WriteFile(ls.path, ls.render(), 0o644); err != nil {
		return fmt.Errorf("failed to save links.yml file: %w", err)
	}
	return nil
}

func (ls *Links) render() []byte {
	if len(ls.links) == 0 {
		return []byte("{}\n")
	}
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range ls.names() {
		l := ls.links[name]
		sec := &yaml.Node{Kind: yaml.MappingNode}
		for _, t := range LinkTypes {
			if to, ok := l.To(t); ok {
				sec.Content = append(sec.Content, scalar(t.String()), scalar(to))
			}
		}
		root.Content = append(root.Content, scalar(encodeKey(name)), sec)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil
	}
	_ = enc.Close()
	return buf.Bytes()
}

func scalar(s string) *yaml.Node {
	n := &yaml.Node{}
	_ = n.Encode(s)
	return n
}

// names answers the linked worlds sorted as Java's TreeMap sorts them.
func (ls *Links) names() []string {
	names := make([]string, 0, len(ls.links))
	for n := range ls.links {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// Bukkit's YAML reads a dot in a key as a section separator.
func encodeKey(name string) string { return strings.ReplaceAll(name, ".", "[dot]") }
func decodeKey(key string) string  { return strings.ReplaceAll(key, "[dot]", ".") }

// Get answers the links of world, false when it has none. MV: getWorldLink
func (ls *Links) Get(world string) (*WorldLink, bool) {
	l, ok := ls.links[world]
	return l, ok
}

// To answers where world's portals of type t lead, false when unset.
func (ls *Links) To(world string, t LinkType) (string, bool) {
	if l, ok := ls.links[world]; ok {
		return l.To(t)
	}
	return "", false
}

// All answers every world's links, in world name order.
func (ls *Links) All() []*WorldLink {
	out := make([]*WorldLink, 0, len(ls.links))
	for _, n := range ls.names() {
		out = append(out, ls.links[n])
	}
	return out
}

// Map answers each linked world's destination for t. MV: getLinksMapForType
func (ls *Links) Map(t LinkType) map[string]string {
	out := map[string]string{}
	for n, l := range ls.links {
		if to, ok := l.To(t); ok {
			out[n] = to
		}
	}
	return out
}

// Add links from's portals of type t to to, in memory. MV: addWorldLink
func (ls *Links) Add(from, to string, t LinkType) {
	l, ok := ls.links[from]
	if !ok {
		l = &WorldLink{From: from}
		ls.links[from] = l
	}
	l.links[t] = to
}

// Remove unlinks from's portals of type t, in memory; false when there was
// no such link. MV: removeWorldLink
func (ls *Links) Remove(from string, t LinkType) bool {
	l, ok := ls.links[from]
	if !ok || l.links[t] == "" {
		return false
	}
	l.links[t] = ""
	if !l.HasLinks() {
		delete(ls.links, from)
	}
	return true
}

// importLegacy adds the links of a Multiverse-NetherPortals 4 config.yml's
// worlds.<world>.portalgoesto sections that links.yml lacks, and answers
// how many it added. A world name with dots is nested sections there.
// MV: LinksManager.migrateLegacyWorldSections
func (ls *Links) importLegacy(worlds map[string]any, parent string) int {
	n := 0
	for key, v := range worlds {
		name := parent + key
		world, _ := v.(map[string]any)
		goesTo, _ := world["portalgoesto"].(map[string]any)
		if len(goesTo) == 0 {
			n += ls.importLegacy(world, name+".")
			continue
		}
		nether, _ := goesTo["NETHER"].(string)
		end, ok := goesTo["ENDER"].(string)
		if !ok {
			end, _ = goesTo["END"].(string)
		}
		for t, to := range map[LinkType]string{Nether: nether, End: end} {
			if _, set := ls.To(name, t); to != "" && !set {
				ls.Add(name, to, t)
				n++
			}
		}
	}
	return n
}
