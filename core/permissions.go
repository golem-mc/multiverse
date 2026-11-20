package core

import (
	"strings"

	"github.com/golem-mc/golem/plugin"
)

// Permission node prefixes. MV: CorePermissions
const (
	PermJoinLocationBypass = "mv.bypass.joinlocation"
	PermWorldAccess        = "multiverse.access"
	PermWorldExempt        = "multiverse.exempt"
	PermGameModeBypass     = "mv.bypass.gamemode"
	PermPlayerLimitBypass  = "mv.bypass.playerlimit"
	PermTeleport           = "multiverse.teleport"
	PermSpawn              = "multiverse.core.spawn"
)

// Defaults of a node, as golem's operator levels: Bukkit's OP is level 1
// and up, FALSE nobody without a provider granting it.
var (
	DefaultOp    = plugin.PermissionDefault{OpLevel: 1}
	DefaultFalse = plugin.PermissionDefault{OpLevel: 5}
	DefaultTrue  = plugin.PermissionDefault{OpLevel: 0}
)

// Permissions registers Multiverse's nodes with the server, each with the
// wildcard parents Bukkit's registerPermissionWithWildcards gives it
// (multiverse.access.* grants every multiverse.access.<world>).
// MV: CorePermissions, PermissionUtils
type Permissions struct {
	core      *Core
	wildcards map[string]map[string]bool
}

func newPermissions(c *Core) *Permissions {
	p := &Permissions{core: c, wildcards: map[string]map[string]bool{}}
	p.Register(PermJoinLocationBypass, DefaultFalse)
	p.Register(Concat(PermSpawn, "self"), DefaultOp)
	p.Register(Concat(PermSpawn, "other"), DefaultOp)
	return p
}

// Register adds node with default def and makes each "prefix.*" above it
// grant it.
func (p *Permissions) Register(node string, def plugin.PermissionDefault) {
	reg := p.core.Server.Permissions()
	reg.Register(plugin.Permission{Name: node, Default: def})
	parts := strings.Split(node, ".")
	for i := 1; i < len(parts); i++ {
		wildcard := strings.Join(parts[:i], ".") + ".*"
		children := p.wildcards[wildcard]
		if children == nil {
			children = map[string]bool{}
			p.wildcards[wildcard] = children
		}
		if children[node] {
			continue
		}
		children[node] = true
		reg.Register(plugin.Permission{Name: wildcard, Default: DefaultFalse, Children: children})
	}
}

// Concat joins permission parts with dots. MV: PermissionUtils.concatPermission
func Concat(parts ...string) string { return strings.Join(parts, ".") }

func (p *Permissions) addWorld(w *World) {
	name := w.Name()
	p.Register(Concat(PermWorldAccess, name), DefaultOp)
	p.Register(Concat(PermWorldExempt, name), DefaultOp)
	p.Register(Concat(PermGameModeBypass, name), DefaultFalse)
	p.Register(Concat(PermPlayerLimitBypass, name), DefaultFalse)
	p.Register(Concat(PermSpawn, name), DefaultOp)
	p.Register(Concat(PermSpawn, "self", name), DefaultOp)
	p.Register(Concat(PermSpawn, "other", name), DefaultOp)
}

// removeWorld leaves the nodes registered: golem's registry has no removal,
// and a node for a world that is gone grants nothing.
func (p *Permissions) removeWorld(*World) {}
