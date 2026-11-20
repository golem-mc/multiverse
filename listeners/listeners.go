// Package listeners applies Multiverse's world properties to the server:
// game mode and flight, respawn, hunger, healing, PvP, mob spawning,
// weather, the chat prefix, advancements and portal forms. MV: listeners,
// EnforcementHandler, WorldConfigNodes' change hooks
package listeners

import "github.com/golem-mc/multiverse/core"

// all registers each file's listeners and hooks; each file adds its own in
// init.
var all []func(c *core.Core)

// Register registers every listener and property hook with c.
func Register(c *core.Core) {
	for _, register := range all {
		register(c)
	}
}
