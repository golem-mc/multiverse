package portals

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/golem/registry"
	"go.yaml.in/yaml/v4"
)

// Manager holds the portals and portals.yml. MV: PortalManager
type Manager struct {
	p    *Portals
	path string
	// order is the portals' order in portals.yml, those that did not load
	// included.
	order   []string
	portals map[string]*Portal
	// unloaded keeps the sections of portals whose location is invalid, so
	// they are written back as they were.
	unloaded map[string]map[string]any
	// chunks finds the portals in a chunk of a world, by world name.
	chunks map[string]map[plugin.ChunkPos][]*Portal
}

func newManager(p *Portals) *Manager {
	return &Manager{p: p, path: filepath.Join(p.Server.DataDir(), "portals.yml")}
}

// Load reads portals.yml, skipping portals whose location is invalid, and
// writes it back. MV: MultiversePortals.loadPortals
func (m *Manager) Load() error {
	m.order, m.portals, m.unloaded = nil, map[string]*Portal{}, map[string]map[string]any{}
	m.chunks = map[string]map[plugin.ChunkPos][]*Portal{}
	data, err := os.ReadFile(m.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	for name, section := range portalSections(&doc) {
		m.order = append(m.order, name)
		pt := m.p.newPortal(name, section)
		pt.region, pt.valid = m.p.parseRegion(pt.Props.String(propLocation))
		if !pt.valid {
			m.p.Log.Warn("Portal '" + name + "' not loaded due to invalid location!")
			m.unloaded[name] = section
			continue
		}
		if _, ok := pt.World(); !ok {
			m.p.Log.Warn("Cannot add portal " + name + " as it location is not in a Multiverse world!")
			m.unloaded[name] = section
			continue
		}
		m.portals[name] = pt
		m.index(pt)
	}
	m.p.Log.Info(strconv.Itoa(len(m.portals)) + " - Portals(s) loaded")
	return m.Save()
}

// portalSections walks the "portals" mapping of a portals.yml document in
// file order.
func portalSections(doc *yaml.Node) func(yield func(string, map[string]any) bool) {
	return func(yield func(string, map[string]any) bool) {
		if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
			return
		}
		root := doc.Content[0]
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value != "portals" || root.Content[i+1].Kind != yaml.MappingNode {
				continue
			}
			ps := root.Content[i+1]
			for j := 0; j+1 < len(ps.Content); j += 2 {
				section := map[string]any{}
				_ = ps.Content[j+1].Decode(&section)
				if !yield(ps.Content[j].Value, section) {
					return
				}
			}
		}
	}
}

// Save writes portals.yml. MV: MultiversePortals.savePortalsConfig
func (m *Manager) Save() error {
	ps := &yaml.Node{Kind: yaml.MappingNode}
	for _, name := range m.order {
		var section *yaml.Node
		if pt, ok := m.portals[name]; ok {
			section = pt.Props.YAML(false)
		} else if raw, ok := m.unloaded[name]; ok {
			section = &yaml.Node{}
			if err := section.Encode(raw); err != nil {
				continue
			}
		} else {
			continue
		}
		ps.Content = append(ps.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: name}, section)
	}
	root := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: "portals"}, ps}}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.path, buf.Bytes(), 0o644)
}

// save writes portals.yml, logging a failure.
func (m *Manager) save() bool {
	if err := m.Save(); err != nil {
		m.p.Log.Error("Failed to save Portals portals.yml: " + err.Error())
		return false
	}
	return true
}

// Get answers the portal named name.
func (m *Manager) Get(name string) (*Portal, bool) {
	pt, ok := m.portals[name]
	return pt, ok
}

// GetFor answers the portal named name if sender may access it.
// MV: PortalManager.getPortal(String, CommandSender)
func (m *Manager) GetFor(name string, sender plugin.CommandSender) (*Portal, bool) {
	if !sender.HasPermission(PermAccess + "." + name) {
		return nil, false
	}
	return m.Get(name)
}

// All answers every portal, by name.
func (m *Manager) All() []*Portal {
	out := make([]*Portal, 0, len(m.portals))
	for _, pt := range m.portals {
		out = append(out, pt)
	}
	slices.SortFunc(out, func(a, b *Portal) int { return strings.Compare(a.name, b.name) })
	return out
}

// For answers the portals sender may see: all for the console, else those
// they may access when enforce-portal-access is on. MV: PortalManager.getPortals
func (m *Manager) For(sender plugin.CommandSender) []*Portal {
	all := m.All()
	if _, ok := sender.(plugin.Player); !ok || !m.p.Config.Bool("portal-usage.enforce-portal-access") {
		return all
	}
	return slices.DeleteFunc(all, func(pt *Portal) bool { return !pt.CanEnter(sender) })
}

// Create adds a new portal named name at r in w, owned by owner; false
// when the name is taken. MV: PortalManager.addPortal
func (m *Manager) Create(name, owner string, r Region) (*Portal, bool) {
	if _, ok := m.portals[name]; ok {
		return nil, false
	}
	pt := m.p.newPortal(name, nil)
	pt.set(propOwner, owner)
	pt.set(propLocation, r.String())
	m.portals[name] = pt
	if _, ok := m.unloaded[name]; ok {
		// A section that did not load is replaced, where it was.
		delete(m.unloaded, name)
	} else {
		m.order = append(m.order, name)
	}
	m.index(pt)
	m.save()
	return pt, true
}

// Remove removes the portal named name; with fromFile, from portals.yml
// too, and clear-on-remove empties its nether portal blocks.
// MV: PortalManager.removePortal
func (m *Manager) Remove(name string, fromFile bool) (*Portal, bool) {
	pt, ok := m.portals[name]
	if !ok {
		return nil, false
	}
	delete(m.portals, name)
	m.unindex(pt)
	if fromFile {
		m.order = slices.DeleteFunc(m.order, func(n string) bool { return n == name })
		m.save()
	}
	if m.p.Config.Bool("portal-creation.clear-on-remove") {
		m.replaceIn(pt, registry.BlockNetherPortal, registry.BlockAir)
	}
	return pt, true
}

// replaceIn sets every block of from in the portal's box to to, without
// physics. MV: PortalManager.replaceInRegion
func (m *Manager) replaceIn(pt *Portal, from, to registry.Block) {
	w, ok := pt.World()
	if !ok {
		return
	}
	h, ok := w.Handle()
	if !ok {
		return
	}
	lo, hi := pt.region.MinBlock(), pt.region.MaxBlock()
	for x := lo.X; x <= hi.X; x++ {
		for y := lo.Y; y <= hi.Y; y++ {
			for z := lo.Z; z <= hi.Z; z++ {
				pos := plugin.BlockPos{X: x, Y: y, Z: z}
				if h.BlockState(pos).Block() == from {
					h.SetBlockState(pos, plugin.DefaultState(to), plugin.UpdateClients)
				}
			}
		}
	}
}

// At answers the portal whose box holds loc. MV: PortalManager.getPortal(Location)
func (m *Manager) At(loc plugin.Location) (*Portal, bool) {
	if loc.World == nil {
		return nil, false
	}
	w, ok := m.p.Core.Worlds.Of(loc.World)
	if !ok {
		return nil, false
	}
	pos := loc.Pos.Block()
	for _, pt := range m.chunks[w.Name()][plugin.ChunkPos{X: pos.X >> 4, Z: pos.Z >> 4}] {
		if pt.valid && pt.region.Contains(w, pos) {
			return pt, true
		}
	}
	return nil, false
}

// index adds pt to the chunks its box covers.
func (m *Manager) index(pt *Portal) {
	if !pt.valid || pt.region.World == nil || m.portals[pt.name] != pt {
		return
	}
	byChunk := m.chunks[pt.region.World.Name()]
	if byChunk == nil {
		byChunk = map[plugin.ChunkPos][]*Portal{}
		m.chunks[pt.region.World.Name()] = byChunk
	}
	for _, c := range chunksOf(pt.region) {
		byChunk[c] = append(byChunk[c], pt)
	}
}

// unindex removes pt from the chunk index.
func (m *Manager) unindex(pt *Portal) {
	if !pt.valid || pt.region.World == nil {
		return
	}
	byChunk := m.chunks[pt.region.World.Name()]
	for _, c := range chunksOf(pt.region) {
		byChunk[c] = slices.DeleteFunc(byChunk[c], func(x *Portal) bool { return x == pt })
	}
}

func chunksOf(r Region) []plugin.ChunkPos {
	lo, hi := r.MinBlock(), r.MaxBlock()
	var out []plugin.ChunkPos
	for cx := lo.X >> 4; cx <= hi.X>>4; cx++ {
		for cz := lo.Z >> 4; cz <= hi.Z>>4; cz++ {
			out = append(out, plugin.ChunkPos{X: cx, Z: cz})
		}
	}
	return out
}
