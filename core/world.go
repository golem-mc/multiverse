package core

import (
	"math"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// World is a world Multiverse knows: one of worlds.yml's entries, loaded or
// not. MV: MultiverseWorld and LoadedMultiverseWorld
type World struct {
	core  *Core
	ref   WorldRef
	Props *Values
	// handle is the loaded world; nil while unloaded.
	handle plugin.World
	// propertyHooks apply a changed property to the loaded world, by path.
	propertyHooks map[string]func()
}

func (c *Core) newWorld(ref WorldRef) *World {
	w := &World{core: c, ref: ref, propertyHooks: map[string]func(){}}
	for path, hook := range c.Worlds.hooks {
		w.propertyHooks[path] = func() { hook(w) }
	}
	w.Props = NewValues(NewNodes(w.worldNodes()...))
	for _, extra := range c.Worlds.extraNodes {
		w.Props.Nodes().Add(extra(w))
	}
	return w
}

// Ref answers how the world is named.
func (w *World) Ref() WorldRef { return w.ref }

// Name answers the world's name. MV: getName
func (w *World) Name() string { return w.ref.Name }

// Key answers the world's dimension key. MV: getKey
func (w *World) Key() plugin.Key { return w.ref.Key }

// Loaded reports whether the world is loaded.
func (w *World) Loaded() bool { return w.handle != nil }

// Handle answers the loaded world.
func (w *World) Handle() (plugin.World, bool) {
	if !w.Loaded() {
		return nil, false
	}
	return w.handle, true
}

// Alias answers the alias, with its colour codes.
func (w *World) Alias() string { return w.Props.String(PropAlias) }

// AliasOrName answers the alias, or the name when there is none.
// MV: getAliasOrName
func (w *World) AliasOrName() string {
	if a := w.Alias(); a != "" {
		return a
	}
	return w.Name()
}

// ColourlessAlias answers AliasOrName without colour codes.
func (w *World) ColourlessAlias() string { return StripColor(w.AliasOrName()) }

// TabCompleteName answers the name commands complete: the colourless alias
// when resolve-alias-name is on. MV: getTabCompleteName
func (w *World) TabCompleteName() string {
	if w.core.Config.Bool("command.resolve-alias-name") {
		return w.ColourlessAlias()
	}
	return w.Name()
}

// Environment answers normal, nether, the_end or custom.
func (w *World) Environment() string { return w.Props.String(PropEnvironment) }

// Hidden reports whether the world is hidden from /mv list.
func (w *World) Hidden() bool { return w.Props.Bool(PropHidden) }

// AutoLoad reports whether the world loads when the server starts.
func (w *World) AutoLoad() bool { return w.Props.Bool(PropAutoLoad) }

// GameMode answers the game mode players get on entering.
func (w *World) GameMode() plugin.GameMode {
	return GameModeOf(w.Props.String(PropGameMode))
}

// Seed answers the seed worlds.yml keeps, and false when it has none.
func (w *World) Seed() (int64, bool) {
	s := w.Props.Long(PropSeed)
	return s, s != math.MinInt64
}

// Generator answers the generator string, "" for the server's.
func (w *World) Generator() string {
	g := w.Props.String(PropGenerator)
	if g == "@error" {
		return ""
	}
	return g
}

// SpawnLocation answers the spawn kept in worlds.yml, nil for none.
func (w *World) SpawnLocation() *SpawnLocation {
	s, _ := w.Props.Custom(PropSpawnLocation).(*SpawnLocation)
	return s
}

// Spawn answers where players spawn in the loaded world: the spawn kept in
// worlds.yml, or the world's own. MV: getSpawnLocation
func (w *World) Spawn() (plugin.Location, bool) {
	h, ok := w.Handle()
	if !ok {
		return plugin.Location{}, false
	}
	if s := w.SpawnLocation(); s != nil {
		return s.Location(h), true
	}
	return h.SpawnPoint(), true
}

// Spawning answers the spawning section.
func (w *World) Spawning() *Spawning {
	s, _ := w.Props.Custom(PropSpawning).(*Spawning)
	if s == nil {
		s = NewSpawning()
		w.Props.setRaw(PropSpawning, s)
	}
	return s
}

// Meta answers the world's metadata, which callers may change and then
// save with Worlds.Save.
func (w *World) Meta() Meta {
	m, _ := w.Props.Custom(PropMeta).(Meta)
	if m == nil {
		m = Meta{}
		w.Props.setRaw(PropMeta, m)
	}
	return m
}

// Players answers the players in the loaded world.
func (w *World) Players() []plugin.Player {
	if h, ok := w.Handle(); ok {
		return h.Players()
	}
	return nil
}

// Set parses and sets a property by name, as /mv modify does, and saves
// worlds.yml.
func (w *World) Set(name, value string) error {
	n, ok := w.Props.Nodes().Find(name)
	if !ok {
		return MsgError(M("mv-core.config.node.notfound", "{node}", name))
	}
	v, err := ParseString(n, value)
	if err != nil {
		return err
	}
	if err := w.Props.Set(n, v); err != nil {
		return err
	}
	return w.core.Worlds.Save()
}

// GameModeOf converts a game mode name to golem's.
func GameModeOf(name string) plugin.GameMode {
	switch strings.ToLower(name) {
	case "creative":
		return plugin.Creative
	case "adventure":
		return plugin.Adventure
	case "spectator":
		return plugin.Spectator
	}
	return plugin.Survival
}

// DifficultyOf converts a difficulty name to golem's.
func DifficultyOf(name string) plugin.Difficulty {
	i := slices.Index(Difficulties, strings.ToLower(name))
	if i < 0 {
		return plugin.DifficultyNormal
	}
	return plugin.Difficulty(i)
}

// EnvironmentOfDimension answers the Bukkit environment of a dimension
// type. MV: World.getEnvironment
func EnvironmentOfDimension(dim plugin.Key) string {
	switch dim.String() {
	case "minecraft:the_nether":
		return "nether"
	case "minecraft:the_end":
		return "the_end"
	case "minecraft:overworld", "minecraft:overworld_caves":
		return "normal"
	}
	return "custom"
}

// DimensionOfEnvironment answers the dimension type a new world of env gets.
func DimensionOfEnvironment(env string) plugin.Key {
	switch strings.ToLower(env) {
	case "nether":
		return plugin.MinecraftKey("the_nether")
	case "the_end":
		return plugin.MinecraftKey("the_end")
	}
	return plugin.MinecraftKey("overworld")
}
