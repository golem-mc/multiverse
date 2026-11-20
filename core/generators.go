package core

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/golem-mc/golem/plugin"
)

// GeneratorFunc answers a plugin's world generator for world and id (what
// followed the colon in "Plugin:id", "" for none), or nil for an id it does
// not know. Bukkit: JavaPlugin.getDefaultWorldGenerator
type GeneratorFunc func(world plugin.Key, id string) plugin.Generator

var (
	generatorsMu sync.Mutex
	generators   = map[string]generatorEntry{}
)

type generatorEntry struct {
	name string
	fn   GeneratorFunc
}

// RegisterGenerator makes a generator available to /mv create -g
// <name>[:id], as an installed Bukkit generator plugin is. golem has no way
// yet for a plugin to offer generators by name (golem-mc/golem#67), so a
// generator plugin calls this from its init. Safe from any goroutine.
func RegisterGenerator(name string, fn GeneratorFunc) {
	generatorsMu.Lock()
	defer generatorsMu.Unlock()
	generators[strings.ToLower(name)] = generatorEntry{name, fn}
}

// GeneratorNames answers the registered generator plugins' names, sorted.
func GeneratorNames() []string {
	generatorsMu.Lock()
	defer generatorsMu.Unlock()
	var out []string
	for _, e := range generators {
		out = append(out, e.name)
	}
	slices.Sort(out)
	return out
}

// Generator resolves a generator string, "Plugin" or "Plugin:id". MV:
// WorldManager.addGeneratorToCreator (WorldCreator.getGeneratorForName)
func (ws *Worlds) Generator(world plugin.Key, spec string) (plugin.Generator, error) {
	name, id, _ := strings.Cut(spec, ":")
	generatorsMu.Lock()
	e, ok := generators[strings.ToLower(name)]
	generatorsMu.Unlock()
	fn := e.fn
	if !ok {
		return nil, MsgError(Lit(fmt.Sprintf("Could not set generator for world '%s': Plugin '%s' does not exist", world.Path, name)))
	}
	g := fn(world, id)
	if g == nil {
		return nil, MsgError(Lit(fmt.Sprintf("Could not set generator for world '%s': Plugin '%s' does not contain a generator for id '%s'", world.Path, name, id)))
	}
	return g, nil
}
