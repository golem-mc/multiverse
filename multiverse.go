// Package multiverse is Multiverse-Core, the multi-world manager, ported to golem.
package multiverse

import "github.com/golem-mc/golem/plugin"

// Multiverse is the core plugin.
type Multiverse struct {
	server plugin.Server
}

func init() { plugin.Register("multiverse-core", &Multiverse{}) }

// Enable starts the plugin.
func (m *Multiverse) Enable(srv plugin.Server) error {
	m.server = srv
	return nil
}

// Disable stops the plugin.
func (m *Multiverse) Disable() {}
