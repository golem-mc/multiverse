// Package commands holds the /mv subcommands, one file per Multiverse-Core
// command class, written against core's command framework.
package commands

import "github.com/golem-mc/multiverse/core"

// all builds each subcommand; each file adds its own in init.
var all []func(c *core.Core) []*core.Cmd

// Register adds every subcommand to c's /mv.
func Register(c *core.Core) {
	for _, build := range all {
		c.Commands.Add(build(c)...)
	}
}
