package core

import "github.com/golem-mc/golem/plugin"

// MVWorldLoadedEvent fires after a Multiverse world loads.
type MVWorldLoadedEvent struct {
	plugin.Base
	World *World
}

// MVWorldUnloadedEvent fires after a Multiverse world unloads.
type MVWorldUnloadedEvent struct {
	plugin.Base
	World *World
}

// MVWorldCreatedEvent fires after /mv create made a world.
type MVWorldCreatedEvent struct {
	plugin.Base
	World *World
}

// MVWorldImportedEvent fires after a world is imported.
type MVWorldImportedEvent struct {
	plugin.Base
	World *World
}

// MVWorldRemovedEvent fires after a world is removed from worlds.yml.
type MVWorldRemovedEvent struct {
	plugin.Base
	Name string
}

// MVWorldDeleteEvent fires before a world's files are deleted; cancelling
// keeps them.
type MVWorldDeleteEvent struct {
	plugin.Cancellable
	World *World
}

// MVWorldClonedEvent fires after a world is cloned.
type MVWorldClonedEvent struct {
	plugin.Base
	World, From *World
}

// MVWorldRegeneratedEvent fires after a world is regenerated.
type MVWorldRegeneratedEvent struct {
	plugin.Base
	World *World
}

// MVWorldPropertyChangedEvent fires after a world property changes; Name is
// the property's name as /mv modify takes it.
type MVWorldPropertyChangedEvent struct {
	plugin.Base
	World    *World
	Name     string
	Old, New any
}

// MVConfigReloadEvent fires after /mv reload read the files again; each
// module appends the files it reloaded, which /mv reload lists.
type MVConfigReloadEvent struct {
	plugin.Base
	Configs []string
}

// MVDebugModeEvent fires when global-debug changes.
type MVDebugModeEvent struct {
	plugin.Base
	Level int
}
