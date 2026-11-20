package core

import (
	"fmt"
	"slices"
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// GeneratorNames answers the enabled plugins that publish world generators
// (plugin.GeneratorProvider), sorted. MV: GeneratorProvider.getGenerators
func (c *Core) GeneratorNames() []string {
	var out []string
	for _, p := range c.Server.Plugins() {
		if p.Enabled && p.Generators {
			out = append(out, p.Name)
		}
	}
	slices.Sort(out)
	return out
}

// Generator resolves a generator string, "Plugin" or "Plugin:id". MV:
// WorldManager.addGeneratorToCreator (WorldCreator.getGeneratorForName)
func (ws *Worlds) Generator(world plugin.Key, spec string) (plugin.Generator, error) {
	name, _, _ := strings.Cut(spec, ":")
	if !slices.ContainsFunc(ws.core.GeneratorNames(), func(n string) bool { return strings.EqualFold(n, name) }) {
		return nil, MsgError(Lit(fmt.Sprintf("Could not set generator for world '%s': Plugin '%s' does not exist", world.Path, name)))
	}
	g, err := ws.core.Server.Generator(world, spec)
	if err != nil {
		return nil, MsgError(Lit(fmt.Sprintf("Could not set generator for world '%s': %s", world.Path, err)))
	}
	return g, nil
}
