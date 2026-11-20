package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"go.yaml.in/yaml/v4"
)

// Anchor is a named location, a teleport destination. MV: MultiverseAnchor
type Anchor struct {
	Name     string
	Location WorldLocation
}

// Anchors keeps anchors.yml. MV: AnchorManager
type Anchors struct {
	core *Core
	path string
	list []*Anchor
}

func newAnchors(c *Core) *Anchors {
	return &Anchors{core: c, path: filepath.Join(c.Server.DataDir(), "anchors.yml")}
}

// Load reads anchors.yml, skipping entries that are not a location.
// MV: AnchorManager.loadAnchors
func (a *Anchors) Load() error {
	a.list = nil
	data, err := os.ReadFile(a.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var doc struct {
		Anchors yaml.Node `yaml:"anchors"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	n := doc.Anchors
	for i := 0; n.Kind == yaml.MappingNode && i+1 < len(n.Content); i += 2 {
		name, value := n.Content[i].Value, n.Content[i+1].Value
		loc, ok := ParseLocationString(value)
		if !ok {
			a.core.Log.Warn("The location for anchor '" + name + "' is INVALID.")
			continue
		}
		a.list = append(a.list, &Anchor{Name: name, Location: loc})
	}
	return nil
}

// Save writes anchors.yml.
func (a *Anchors) Save() error {
	m := &yaml.Node{Kind: yaml.MappingNode}
	for _, an := range a.list {
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: an.Name},
			&yaml.Node{Kind: yaml.ScalarNode, Value: an.Location.String()})
	}
	root := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "anchors"}, m}}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(a.path, buf.Bytes(), 0o644)
}

// Get answers the anchor with name (exact case).
func (a *Anchors) Get(name string) (*Anchor, bool) {
	for _, an := range a.list {
		if an.Name == name {
			return an, true
		}
	}
	return nil, false
}

// All answers every anchor.
func (a *Anchors) All() []*Anchor { return slices.Clone(a.list) }

// For answers the anchors sender may see: all for the console, those in
// worlds a player can access (or unknown worlds) for a player.
// MV: AnchorManager.getAnchors
func (a *Anchors) For(sender plugin.CommandSender) []*Anchor {
	if _, ok := sender.(plugin.Player); !ok {
		return a.All()
	}
	var out []*Anchor
	for _, an := range a.list {
		if w, ok := a.core.Worlds.Get(an.Location.World); !ok || a.core.CanAccess(sender, w) {
			out = append(out, an)
		}
	}
	return out
}

// Set adds or moves an anchor and saves. MV: AnchorManager.setAnchor
func (a *Anchors) Set(name string, loc WorldLocation) error {
	if an, ok := a.Get(name); ok {
		an.Location = loc
	} else {
		a.list = append(a.list, &Anchor{Name: name, Location: loc})
	}
	return a.Save()
}

// Delete removes an anchor and saves. MV: AnchorManager.deleteAnchor
func (a *Anchors) Delete(name string) error {
	n := len(a.list)
	a.list = slices.DeleteFunc(a.list, func(an *Anchor) bool { return an.Name == name })
	if len(a.list) == n {
		return errors.New("anchor does not exist")
	}
	return a.Save()
}

// worldNameOf answers the name a location's world goes by in anchors.yml.
func (c *Core) worldNameOf(h plugin.World) string {
	if w, ok := c.Worlds.Of(h); ok {
		return w.Name()
	}
	return strings.ReplaceAll(h.Key().String(), ":", "_")
}
