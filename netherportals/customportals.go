package netherportals

import (
	"slices"
	"sync"

	"github.com/golem-mc/golem/plugin"
)

// CustomPortalCheck reports whether a plugin handles the portal at loc
// itself; entity is the one using it, nil when unknown.
// MV: CustomPortalsHandleCheck
type CustomPortalCheck func(entity plugin.Entity, loc plugin.Location) bool

// CustomPortals holds the checks of plugins that handle some portals
// themselves (Multiverse-Portals' portals): NetherPortals leaves a portal
// any check claims alone. MV: CustomPortalsHandler
type CustomPortals struct {
	mu     sync.Mutex
	checks []*CustomPortalCheck
}

// Register adds a check; the answer removes it.
func (cp *CustomPortals) Register(check CustomPortalCheck) (unregister func()) {
	p := &check
	cp.mu.Lock()
	cp.checks = append(cp.checks, p)
	cp.mu.Unlock()
	return func() {
		cp.mu.Lock()
		cp.checks = slices.DeleteFunc(cp.checks, func(c *CustomPortalCheck) bool { return c == p })
		cp.mu.Unlock()
	}
}

// Handled reports whether a registered check claims the portal at loc.
// MV: CustomPortalsHandler.isHandledByCustomPortals
func (cp *CustomPortals) Handled(entity plugin.Entity, loc plugin.Location) bool {
	cp.mu.Lock()
	checks := slices.Clone(cp.checks)
	cp.mu.Unlock()
	for _, c := range checks {
		if (*c)(entity, loc) {
			return true
		}
	}
	return false
}
