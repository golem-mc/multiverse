package netherportals

import (
	"path/filepath"
	"strings"

	"github.com/golem-mc/multiverse/core"
)

// ConfigVersion is the config.yml version this port writes.
const ConfigVersion = 5.1

// Config paths. MV: NetherPortalsConfigNodes
const (
	PathNetherPrefix              = "portal-auto-link-when.nether.prefix"
	PathNetherSuffix              = "portal-auto-link-when.nether.suffix"
	PathEndPrefix                 = "portal-auto-link-when.end.prefix"
	PathEndSuffix                 = "portal-auto-link-when.end.suffix"
	PathHandleEndExitRespawn      = "handle-end-exit-respawn"
	PathBounceback                = "bounceback"
	PathTeleportEntities          = "teleport-entities"
	PathSendDisabledPortalMessage = "send-disabled-portal-message"
	PathSendNoDestinationMessage  = "send-no-destination-message"
	PathEndPlatformDropBlocks     = "end-platform-drop-blocks"
)

// configNodes is config.yml, in Multiverse-NetherPortals' order and with
// its comments. MV: NetherPortalsConfigNodes
func configNodes() []*core.Node {
	return []*core.Node{
		{Path: "portal-auto-link-when", Kind: core.KindHeader, Comments: []string{
			"####################################################################################################",
			"#                                                                                                  #",
			"#                         MULTIVERSE-NETHERPORTALS CONFIGURATION                                   #",
			"#                                                                                                  #",
			"#    WIKI:        https://mvplugins.org/netherportals/                                             #",
			"#    DISCORD:     https://discord.gg/NZtfKky                                                       #",
			"#    BUG REPORTS: https://github.com/Multiverse/Multiverse-NetherPortals/issues                    #",
			"#    DONATE:      https://github.com/sponsors/Multiverse                                           #",
			"#                                                                                                  #",
			"#    New options are added to this file automatically. If you manually made changes                #",
			"#    while your server is running, use Multiverse-Core's reload command.                           #",
			"#                                                                                                  #",
			"####################################################################################################",
			"",
			"Controls the prefix and suffix used to automatically find linked dimension worlds.",
			"For example, by default the world 'cat_nether' nether portal will be linked to world 'cat'.",
		}},
		{Path: PathNetherPrefix, Name: "nether-prefix", Kind: core.KindString, Default: ""},
		{Path: PathNetherSuffix, Name: "nether-suffix", Kind: core.KindString, Default: "_nether"},
		{Path: PathEndPrefix, Name: "end-prefix", Kind: core.KindString, Default: ""},
		{Path: PathEndSuffix, Name: "end-suffix", Kind: core.KindString, Default: "_the_end"},
		{Path: PathHandleEndExitRespawn, Kind: core.KindBool, Default: true, Comments: []string{
			"",
			"When enabled, end portal exits from end world will override default behaviour and respawn players",
			"to the linked world's spawn. This is to handle the special case where player exiting end portal",
			"in end are treated as respawns instead of portal teleports in Minecraft.",
			"** This feature is only supported on Paper servers.",
		}},
		{Path: PathBounceback, Kind: core.KindBool, Default: true, Comments: []string{
			"",
			"When enabled, players are pushed out of a portal when its destination is unavailable.",
		}},
		{Path: PathTeleportEntities, Kind: core.KindBool, Default: true, Comments: []string{
			"",
			"When enabled, non-player entities can travel through Nether and End portals.",
		}},
		{Path: PathSendDisabledPortalMessage, Kind: core.KindBool, Default: true, Comments: []string{
			"",
			"When enabled, players are told when portals are disabled in the current world.",
		}},
		{Path: PathSendNoDestinationMessage, Kind: core.KindBool, Default: true, Comments: []string{
			"",
			"When enabled, players are told when a portal has no valid destination.",
		}},
		{Path: PathEndPlatformDropBlocks, Kind: core.KindBool, Default: true, Comments: []string{
			"",
			"When enabled, blocks replaced while creating an End platform drop as items on non-default worlds to",
			"mimic vanilla Minecraft behaviour.",
			"Note: The default end is handled by the server and loaded even without Multiverse installed, thus",
			"      multiverse will have no control over it's drop items behaviour.",
		}},
		{Path: "version", Kind: core.KindDouble, Default: 0.0, Hidden: true, Comments: []string{
			"",
			"",
			"The configuration version. Do not edit this value.",
		}},
	}
}

// newConfig answers config.yml in dir; legacy receives a Multiverse-NetherPortals
// 4 file's worlds section and reports whether it kept its links, the save then
// leaving the section out. MV: NetherPortalsConfig
func newConfig(dir string, legacy func(worlds map[string]any) bool) *core.Config {
	return core.NewConfigFile(filepath.Join(dir, "config.yml"), configNodes(), func(m map[string]any) {
		migrateConfig(m)
		if worlds, ok := m["worlds"].(map[string]any); ok && legacy(worlds) {
			delete(m, "worlds")
		}
	}, ConfigVersion)
}

// migrateConfig moves a config.yml before version 5.1 to its keys; an empty
// file is new and has nothing to move. MV: NetherPortalsConfig's ConfigMigrator
func migrateConfig(m map[string]any) {
	if len(m) == 0 {
		return
	}
	var v float64
	switch x := m["version"].(type) {
	case float64:
		v = x
	case int:
		v = float64(x)
	}
	if v >= 5.1 {
		return
	}
	m[PathHandleEndExitRespawn] = false
	for _, key := range []string{"teleport_entities", "send_disabled_portal_message", "send_no_destination_message", "end_platform_drop_blocks"} {
		if val, ok := m[key]; ok {
			delete(m, key)
			m[strings.ReplaceAll(key, "_", "-")] = val
		}
	}
	m["version"] = 5.1
}
