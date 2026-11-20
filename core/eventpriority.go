package core

import (
	"slices"

	"github.com/golem-mc/golem/plugin"
)

// EventPriority answers the listener priority config.yml sets under
// event-priority.<name>, e.g. "player-respawn". It is read when the
// listener registers, so a change applies after a restart. MV:
// EventPriorityMapper
func (c *Core) EventPriority(name string) plugin.Priority {
	i := slices.Index(eventPriorities, c.Config.String("event-priority."+name))
	if i < 0 {
		return plugin.Normal
	}
	return plugin.Lowest + plugin.Priority(i)
}
