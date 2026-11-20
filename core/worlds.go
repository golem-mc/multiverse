package core

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
	"go.yaml.in/yaml/v4"
)

// Worlds is the world manager: worlds.yml's worlds, loaded or not. Every
// method runs on the tick goroutine. MV: WorldManager, WorldStore,
// WorldsConfigManager
type Worlds struct {
	core *Core
	path string
	list []*World
	// hooks apply a changed property to a loaded world, by property path;
	// the listeners that enforce a property register theirs.
	hooks map[string]func(*World)
	// extraNodes add properties of a module's to every world.
	extraNodes []func(*World) *Node
	loading    map[string]bool
}

func newWorlds(c *Core, dir string) *Worlds {
	return &Worlds{core: c, path: filepath.Join(dir, "worlds.yml"), hooks: map[string]func(*World){},
		loading: map[string]bool{}}
}

// OnChange registers fn to run when the property at path of a loaded world
// changes, and when the world loads.
func (ws *Worlds) OnChange(path string, fn func(*World)) {
	prev := ws.hooks[path]
	ws.hooks[path] = func(w *World) {
		if prev != nil {
			prev(w)
		}
		if w.Loaded() {
			fn(w)
		}
	}
	for _, w := range ws.list {
		w.propertyHooks[path] = func() { ws.hooks[path](w) }
	}
}

// All answers every world, loaded or not, in worlds.yml's order.
func (ws *Worlds) All() []*World { return slices.Clone(ws.list) }

// Loaded answers the loaded worlds.
func (ws *Worlds) Loaded() []*World {
	var out []*World
	for _, w := range ws.list {
		if w.Loaded() {
			out = append(out, w)
		}
	}
	return out
}

// Unloaded answers the worlds that are known but not loaded.
func (ws *Worlds) Unloaded() []*World {
	var out []*World
	for _, w := range ws.list {
		if !w.Loaded() {
			out = append(out, w)
		}
	}
	return out
}

// Get answers the world by name, legacy name or key, ignoring case.
// MV: WorldManager.getWorld
func (ws *Worlds) Get(nameOrKey string) (*World, bool) {
	for _, w := range ws.list {
		if strings.EqualFold(w.Name(), nameOrKey) || strings.EqualFold(w.Key().String(), nameOrKey) ||
			(w.Props.String(PropLegacyWorldName) != "" && strings.EqualFold(w.Props.String(PropLegacyWorldName), nameOrKey)) {
			return w, true
		}
	}
	if ref, err := ws.core.ParseWorldRef(nameOrKey); err == nil {
		for _, w := range ws.list {
			if w.Key() == ref.Key {
				return w, true
			}
		}
	}
	return nil, false
}

// GetByNameOrAlias answers the world by name, or by its colourless alias
// when resolve-alias-name is on. MV: getWorldByNameOrAlias
func (ws *Worlds) GetByNameOrAlias(s string) (*World, bool) {
	if w, ok := ws.Get(s); ok {
		return w, true
	}
	if !ws.core.Config.Bool("command.resolve-alias-name") {
		return nil, false
	}
	for _, w := range ws.list {
		if w.Alias() != "" && strings.EqualFold(StripColor(w.Alias()), s) {
			return w, true
		}
	}
	return nil, false
}

// GetLoaded answers a loaded world by name or alias.
func (ws *Worlds) GetLoaded(s string) (*World, bool) {
	w, ok := ws.GetByNameOrAlias(s)
	if !ok || !w.Loaded() {
		return nil, false
	}
	return w, true
}

// Of answers the Multiverse world of a loaded golem world.
func (ws *Worlds) Of(h plugin.World) (*World, bool) {
	if h == nil {
		return nil, false
	}
	key := h.Key()
	for _, w := range ws.list {
		if w.Key() == key && w.Loaded() {
			return w, true
		}
	}
	return nil, false
}

// Default answers the overworld's Multiverse world. MV: getDefaultWorld
func (ws *Worlds) Default() (*World, bool) {
	return ws.Of(ws.core.Server.Overworld())
}

// Init reads worlds.yml, imports the server's worlds Multiverse does not
// know, loads the auto-load worlds and saves. MV: WorldManager.initAllWorlds
func (ws *Worlds) Init() error {
	if err := ws.loadConfig(); err != nil {
		return err
	}
	ws.importExisting()
	for _, w := range ws.list {
		if !w.Loaded() && w.AutoLoad() {
			if _, err := ws.Load(w); err != nil {
				ws.core.Log.Error(fmt.Sprintf("Failed to autoload world '%s': %s", w.Name(), StripColor(english.Format(nil, ErrorMessage(err)))))
			}
		}
	}
	return ws.Save()
}

// loadConfig reads worlds.yml, adding worlds it newly names and dropping
// those it no longer does (unloading them). MV: WorldsConfigManager.load
func (ws *Worlds) loadConfig() error {
	data, err := os.ReadFile(ws.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m := map[string]any{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("worlds.yml: %w", err)
	}
	var keep []*World
	for _, rawKey := range orderedKeys(data) {
		sec, ok := m[rawKey].(map[string]any)
		if !ok {
			continue
		}
		ref, err := ws.core.ParseWorldRef(strings.ReplaceAll(rawKey, "[dot]", "."))
		if err != nil {
			ws.core.Log.Warn(fmt.Sprintf("Failed to parse world key '%s' in config: %s", rawKey, err))
			continue
		}
		w, ok := ws.byRef(ref)
		if !ok {
			w = ws.core.newWorld(ref)
		}
		w.Props.Load(sec)
		keep = append(keep, w)
	}
	for _, w := range ws.list {
		if !slices.Contains(keep, w) && w.Loaded() {
			if h, ok := w.Handle(); ok {
				_ = ws.core.Server.UnloadWorld(h, true)
			}
		}
	}
	ws.list = keep
	return nil
}

func (ws *Worlds) byRef(ref WorldRef) (*World, bool) {
	for _, w := range ws.list {
		if w.Key() == ref.Key {
			return w, true
		}
	}
	return nil, false
}

// orderedKeys answers the top-level keys of a YAML document in file order.
func orderedKeys(data []byte) []string {
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	var keys []string
	for i := 0; i+1 < len(doc.Content[0].Content); i += 2 {
		keys = append(keys, doc.Content[0].Content[i].Value)
	}
	return keys
}

// importExisting adds the loaded worlds Multiverse does not know: the
// vanilla dimensions when auto-import-default-worlds is on, the others when
// auto-import-3rd-party-worlds is. MV: WorldManager.importExistingWorlds
func (ws *Worlds) importExisting() {
	for _, h := range ws.core.Server.Worlds() {
		if _, ok := ws.byRef(ws.core.RefOfKey(h.Key())); ok {
			continue
		}
		vanilla := h.Key().Namespace == "minecraft" &&
			slices.Contains([]string{"overworld", "the_nether", "the_end"}, h.Key().Path)
		if vanilla && !ws.core.Config.Bool("world.auto-import-default-worlds") ||
			!vanilla && !ws.core.Config.Bool("world.auto-import-3rd-party-worlds") {
			continue
		}
		w := ws.add(ws.core.RefOfKey(h.Key()))
		w.Props.setRaw(PropGenerator, "")
		w.Props.setRaw(PropBiome, "")
		ws.attach(w, h)
		ws.core.fire(&MVWorldImportedEvent{World: w})
	}
}

// add appends a new world to worlds.yml.
func (ws *Worlds) add(ref WorldRef) *World {
	w := ws.core.newWorld(ref)
	ws.list = append(ws.list, w)
	ws.core.Permissions.addWorld(w)
	return w
}

// attach marks w loaded as h, records what the server says about it and
// applies its properties. MV: LoadedMultiverseWorld's constructor
func (ws *Worlds) attach(w *World, h plugin.World) {
	w.handle = h
	w.Props.setRaw(PropEnvironment, EnvironmentOfDimension(h.Dimension()))
	w.Props.setRaw(PropSeed, h.Seed())
	w.Props.setRaw(PropLegacyWorldName, w.Name())
	if !w.Props.isSet(PropScale) {
		w.Props.setRaw(PropScale, defaultScale(w.Environment()))
	}
	if w.SpawnLocation() == nil {
		w.Props.setRaw(PropSpawnLocation, SpawnLocationOf(h.SpawnPoint()))
	}
	for _, hook := range w.propertyHooks {
		hook()
	}
	ws.core.fire(&MVWorldLoadedEvent{World: w})
}

// Load loads a known world with the options worlds.yml keeps for it. MV:
// WorldManager.loadWorld
func (ws *Worlds) Load(w *World) (*World, error) {
	if w.Loaded() {
		return nil, MsgError(M("mv-core.loadworld.worldexistloaded", "{world}", w.Name()))
	}
	if ws.loading[w.Name()] {
		return nil, MsgError(M("mv-core.loadworld.worldalreadyloading", "{world}", w.Name()))
	}
	ws.loading[w.Name()] = true
	defer delete(ws.loading, w.Name())
	if h, ok := ws.core.Server.World(w.Key()); ok {
		if env := EnvironmentOfDimension(h.Dimension()); env != w.Environment() {
			return nil, MsgError(M("mv-core.loadworld.bukkitenvironmentmismatch", "{world}", w.Name(),
				"{bukkitEnvironment}", strings.ToUpper(env), "{mvEnvironment}", strings.ToUpper(w.Environment())))
		}
		ws.attach(w, h)
		return w, nil
	}
	opts, err := ws.options(w.Key(), w.Environment(), w.Generator())
	if err != nil {
		return nil, err
	}
	if seed, ok := w.Seed(); ok {
		opts.Seed = seed
	}
	h, err := ws.core.Server.CreateWorld(opts)
	if err != nil {
		return nil, MsgError(M("mv-core.worldcreator.bukkitcreationfailed", "{world}", w.Name(), "{error}", err.Error()))
	}
	ws.attach(w, h)
	return w, nil
}

// options builds the golem options for a world of env and generator.
func (ws *Worlds) options(key plugin.Key, env, generator string) (plugin.WorldOptions, error) {
	opts := plugin.WorldOptions{Key: key, Dimension: DimensionOfEnvironment(env)}
	if generator != "" {
		g, err := ws.Generator(key, generator)
		if err != nil {
			return opts, MsgError(M("mv-core.worldcreator.invalidchunkgenerator", "{generator}", generator, "{error}", err.Error()))
		}
		opts.Generator = g
	}
	return opts, nil
}

// Unload unloads a loaded world, saving it unless save is false. Players in
// it must have left first. MV: WorldManager.unloadWorld
func (ws *Worlds) Unload(w *World, save bool) error {
	h, ok := w.Handle()
	if !ok {
		return MsgError(M("mv-core.unloadworld.worldunloaded", "{world}", w.Name()))
	}
	if h.Key() == ws.core.Server.Overworld().Key() {
		return MsgError(M("mv-core.exception.multiverseworld.unloaddefaultworld", "{world}", w.Name()))
	}
	if n := len(h.Players()); n > 0 {
		return MsgError(M("mv-core.exception.multiverseworld.unloadplayersinworld", "{count}", n))
	}
	if err := ws.core.Server.UnloadWorld(h, save); err != nil {
		return MsgError(M("mv-core.unloadworld.bukkitunloadfailed", "{world}", w.Name(), "{error}", err.Error()))
	}
	// The WorldUnloadEvent listener marked it unloaded and fired
	// MVWorldUnloadedEvent.
	return nil
}

// Save writes worlds.yml. MV: WorldsConfigManager.save
func (ws *Worlds) Save() error {
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, w := range ws.list {
		key := strings.ReplaceAll(w.Ref().Serialise(), ".", "[dot]")
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, w.Props.YAML(false))
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}); err != nil {
		return err
	}
	out := buf.Bytes()
	if len(ws.list) == 0 {
		out = []byte("{}\n")
	}
	if err := os.MkdirAll(filepath.Dir(ws.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(ws.path, out, 0o644)
}

// remove drops w from worlds.yml (not its files); it must be unloaded.
func (ws *Worlds) remove(w *World) {
	ws.list = slices.DeleteFunc(ws.list, func(x *World) bool { return x == w })
	ws.core.Permissions.removeWorld(w)
}

// SeedOf answers a world's seed as Multiverse shows it.
func SeedOf(w *World) string {
	if s, ok := w.Seed(); ok {
		return fmt.Sprint(s)
	}
	return fmt.Sprint(int64(math.MinInt64))
}
