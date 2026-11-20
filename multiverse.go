// Package multiverse is Multiverse-Core, the multi-world manager, ported to
// golem: the plugin registered as "multiverse-core". The modules
// (netherportals, portals, inventories) are plugins in their own packages
// that depend on it.
package multiverse

import (
	"github.com/golem-mc/golem/plugin"
	"github.com/golem-mc/multiverse/commands"
	"github.com/golem-mc/multiverse/core"
)

// Multiverse is the core plugin.
type Multiverse struct{ core *core.Core }

func init() { plugin.Register("multiverse-core", &Multiverse{}) }

// Enable loads the configs and worlds, then registers the commands.
func (m *Multiverse) Enable(srv plugin.Server) error {
	c, err := core.Enable(srv)
	if err != nil {
		return err
	}
	m.core = c
	commands.Register(c)
	return c.Commands.Register()
}

// Disable saves worlds.yml.
func (m *Multiverse) Disable() {
	if m.core != nil {
		m.core.Disable()
	}
}
