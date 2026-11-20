package core

import "github.com/golem-mc/golem/plugin"

// listenWorlds keeps the world manager in step with worlds other plugins
// load and unload. MV: MVWorldListener
func (c *Core) listenWorlds() {
	plugin.Listen(c.Server, func(e *plugin.WorldUnloadEvent) {
		if w, ok := c.Worlds.Of(e.World); ok {
			w.handle = nil
			c.fire(&MVWorldUnloadedEvent{World: w})
		}
	}, plugin.ListenOptions{Priority: plugin.Monitor, IgnoreCancelled: true})
	plugin.Listen(c.Server, func(e *plugin.WorldLoadEvent) {
		if w, ok := c.Worlds.byRef(c.RefOfKey(e.World.Key())); ok && !w.Loaded() && !c.Worlds.loading[w.Name()] {
			c.Worlds.attach(w, e.World)
		}
	}, plugin.ListenOptions{Priority: plugin.Monitor})
}
