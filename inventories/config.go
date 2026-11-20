package inventories

import (
	"fmt"
	"path/filepath"

	"github.com/golem-mc/multiverse/core"
)

// configVersion is the config.yml version this port writes.
const configVersion = 5.0

// configNodes is config.yml, in Multiverse-Inventories 5's order and with
// its comments. MV: InventoriesConfigNodes
func (m *Module) configNodes() []*core.Node {
	b := func(path, name string, def bool, comments ...string) *core.Node {
		return &core.Node{Path: path, Name: name, Kind: core.KindBool, Default: def, Comments: comments}
	}
	i := func(path, name string, def int, comments ...string) *core.Node {
		return &core.Node{Path: path, Name: name, Kind: core.KindInt, Default: def, Comments: comments}
	}
	header := func(path string, comments ...string) *core.Node {
		return &core.Node{Path: path, Kind: core.KindHeader, Comments: comments}
	}
	m.firstRun = &core.Node{Path: "first-run", Kind: core.KindBool, Default: true, Hidden: true,
		Comments: []string{"", "", "Do not edit the following values!!!!!"}}
	return []*core.Node{
		header("share-handling",
			"#-----------------------------------------------------------------------------------------------------------------#",
			"#                                                                                                                 #",
			`#     __  __ _   _ _____ _    _____   _____ ___  ___ ___    ___ _  ___   _____ _  _ _____ ___  ___ ___ ___ ___    #`,
			`#    |  \/  | | | |_   _| |  |_ _\ \ / / __| _ \/ __| __|  |_ _| \| \ \ / / __| \| |_   _/ _ \| _ \_ _| __/ __|   #`,
			"#    | |\\/| | |_| | | | | |__ | | \\ V /| _||   /\\__ \\ _|    | || .` |\\ V /| _|| .` | | || (_) |   /| || _|\\__ \\   #",
			`#    |_|  |_|\___/  |_| |____|___| \_/ |___|_|_\|___/___|  |___|_|\_| \_/ |___|_|\_| |_| \___/|_|_\___|___|___/   #`,
			"#                                                                                                                 #",
			"#                                                                                                                 #",
			"#                                                                                                                 #",
			"#                                                                                                                 #",
			"#            WIKI:        https://mvplugins.org/inventories/                                                      #",
			"#            DISCORD:     https://discord.gg/NZtfKky                                                              #",
			"#            BUG REPORTS: https://github.com/Multiverse/Multiverse-Inventories/issues                             #",
			"#            DONATE:      https://github.com/sponsors/Multiverse                                                  #",
			"#                                                                                                                 #",
			"#                                                                                                                 #",
			"#            New options are added to this file automatically. If you manually made changes                       #",
			"#            to this file while your server is running, please run `/mvinv reload` command.                       #",
			"#                                                                                                                 #",
			"#-----------------------------------------------------------------------------------------------------------------#",
			"", ""),
		b("share-handling.enable-bypass-permissions", "enable-bypass", false,
			"If this is set to true, it will enable bypass permissions (Check the wiki for more info.)"),
		b("share-handling.enable-gamemode-share-handling", "enable-gamemode-share-handling", false, "",
			"If this is set to true, players will have different inventories/stats for each game mode.",
			"Please note that old data migrated to the version that has this feature will have their data copied for both game modes."),
		b("share-handling.default-ungrouped-worlds", "default-ungrouped-worlds", false, "",
			"If set to true, any world not listed in a group will automatically be assigned to the 'default' group!"),
		b("share-handling.use-optionals-for-ungrouped-worlds", "use-optionals-for-ungrouped-worlds", true, "",
			"When set to true, optional shares WILL be utilized in cases where a group does not cover their uses for a world.",
			"An example of this in action would be an ungrouped world using last_location. When this is true, players will return to their last location in that world.",
			"When set to false, optional shares WILL NOT be utilized in these cases, effectively disabling it for ungrouped worlds."),
		{Path: "share-handling.active-optional-shares", Kind: core.KindCustom, Hidden: true,
			DefaultFn: func(*core.Values) any { return &Shares{} },
			Decode: func(raw any) any {
				if l, ok := raw.([]any); ok {
					names := make([]string, len(l))
					for i, e := range l {
						names[i] = fmt.Sprint(e)
					}
					return SharesFromList(names)
				}
				return SharesFromList([]string{fmt.Sprint(raw)})
			},
			Encode:   func(v any) any { return v.(*Shares).Strings() },
			OnChange: func(any, any) { m.recalculateEnabledShares() },
			Comments: []string{"", "You must specify optional shares you wish to use here or they will be ignored.",
				`Built-in optional shares are: "economy" and "last_location".`}},
		header("sharables", "", ""),
		b("sharables.validate-bed-anchor-respawn-location", "validate-bed-anchor-respawn-location", true,
			"When enabled, we will validate the bed/anchor respawn location on group/world change.",
			"The validation checks if the bed/anchor block still exists at the saved location and is usable.",
			"When the validation fails, the respawn location will be cleared and default world spawn will be used instead.",
			"Disable this if you don't want validation, or you want another plugin to handle respawn logic."),
		b("sharables.reset-last-location-on-death", "reset-last-location-on-death", false, "",
			"When set to true, the last location of the player will be reset when they die.",
			"This is useful if they respawn in a different world and you do not want them to return to their death location."),
		b("sharables.apply-last-location-for-all-teleports", "apply-last-location-for-all-teleports", true, "",
			"When enabled, the last location of the player will be applied for any teleportation.",
			"This is useful as you want to use the last location for any teleportation, such as the warp system.",
			"When disabled, you can only use `/mv tp ll:worldname` to teleport to the player's last location."),
		b("sharables.use-byte-serialization-for-inventory-data", "use-byte-serialization-for-inventory-data", false, "",
			"When enabled, we will use paper's improved byte serialization for inventory data.",
			"When disabled, we will use the legacy configuration serialization method.",
			"!!!!!BIG NOTE:",
			"  This option is only applicable on PAPERMC.",
			"  Once you enable this option, you cannot change your server software back to SPIGOT.",
			"------------",
			"Byte serialization will use minecraft's NBT format. NBT is safer for data migrations as it will use the built in ",
			"data converter instead of bukkits dangerous serialization system. This will fix various issues with the inventory data",
			"such as Skulker Box data loss, equip-sound crash, FoodEffect error, and more."),
		header("performance", "", ""),
		b("performance.apply-playerdata-on-join", "apply-playerdata-on-join", false,
			"Minecraft will already load the most up-to-date player data and this option will generally be redundant.",
			"The only possible edge case uses is if you have a need to always modify the mvinv playerdata while the player is offline."),
		b("performance.always-write-world-profile", "always-write-world-profile", true, "",
			"By default, even when the group shares all or going to a world within the same group, the world profile will still be written to disk.",
			"This will ensure that the world profile is always up-to-date, so when removing the world from the group, it will not be missing data.",
			"However, if you are certain that your world will always be in a group, you can set this to false to slightly improve performance."),
		header("performance.preload-data-on-join", "",
			"Pre-loads player data into caches when joining the server.",
			"This will reduce the load time on first teleport to the world/group, with the cost of increased memory usage and join time."),
		{Path: "performance.preload-data-on-join.worlds", Name: "preload-data-on-join-worlds", Kind: core.KindList, Default: []string{}},
		{Path: "performance.preload-data-on-join.groups", Name: "preload-data-on-join-groups", Kind: core.KindList, Default: []string{}},
		header("performance.cache", "", "NOTE: Cache options require a server restart to take effect."),
		i("performance.cache.player-file-cache-size", "player-file-cache-size", 2000),
		i("performance.cache.player-file-cache-expiry", "player-file-cache-expiry", 60),
		i("performance.cache.player-profile-cache-size", "player-profile-cache-size", 6000),
		i("performance.cache.player-profile-cache-expiry", "player-profile-cache-expiry", 60),
		i("performance.cache.global-profile-cache-size", "global-profile-cache-size", 500),
		i("performance.cache.global-profile-cache-expiry", "global-profile-cache-expiry", 60),
		header("misc", "", ""),
		b("misc.register-papi-hook", "register-papi-hook", true,
			"This config option defines whether or not Multiverse should register the PlaceholderAPI hook.",
			"This only applies if PlaceholderAPI is installed."),
		header("misc.max-items-size", "",
			"These are the maximum sizes for each inventory type. You should not change these values unless",
			"you have a plugin or feature that changes the size of these inventories.",
			"-------",
			"Changing these values without a plugin or feature that increases the size of these inventories",
			"may result in errors and loss of items when switching worlds/groups. This config option is merely",
			"to resolve compatibility issues with other plugins/features that alter inventory sizes.",
			"-------",
			"One notable example is if you used purpur's config option to increase ender chest size."),
		i("misc.max-items-size.inventory", "max-player-inventory-size", inventorySize),
		i("misc.max-items-size.ender-chest", "max-ender-chest-size", enderChestSize),
		i("misc.max-items-size.armor", "max-inventory-title-length", armorSize),
		m.firstRun,
		{Path: "version", Kind: core.KindDouble, Default: 0.0, Hidden: true},
	}
}

func (m *Module) newConfig() *core.Config {
	return core.NewConfigFile(filepath.Join(m.Server.DataDir(), "config.yml"), m.configNodes(), m.migrateConfig, configVersion)
}

// migrateConfig moves a Multiverse-Inventories 2 to 4 config.yml's keys to
// version 5's, and keeps its groups for groups.yml.
// MV: InventoriesConfig's ConfigMigrator, YamlWorldGroupManager.migrateGroups
func (m *Module) migrateConfig(raw map[string]any) {
	if g, ok := raw["groups"]; ok {
		m.legacyGroups, _ = g.(map[string]any)
		delete(raw, "groups")
	}
	if v, ok := raw["version"]; ok {
		if f, ok := number(v); ok && f >= 5.0 {
			return
		}
	}
	settings, _ := raw["settings"].(map[string]any)
	shares, _ := raw["shares"].(map[string]any)
	section := func(name string) map[string]any {
		s, ok := raw[name].(map[string]any)
		if !ok {
			s = map[string]any{}
			raw[name] = s
		}
		return s
	}
	move := func(from map[string]any, key string, to map[string]any, toKey string) {
		if v, ok := from[key]; ok {
			to[toKey] = v
			delete(from, key)
		}
	}
	if settings != nil {
		if v, ok := settings["first_run"]; ok {
			raw["first-run"] = v
			delete(settings, "first_run")
		}
		move(settings, "use_bypass", section("share-handling"), "enable-bypass-permissions")
		move(settings, "default_ungrouped_worlds", section("share-handling"), "default-ungrouped-worlds")
		move(settings, "save_load_on_log_in_out", section("performance"), "apply-playerdata-on-join")
		move(settings, "use_game_mode_profiles", section("share-handling"), "enable-gamemode-share-handling")
		if len(settings) == 0 {
			delete(raw, "settings")
		}
	}
	if shares != nil {
		move(shares, "optionals_for_ungrouped_worlds", section("share-handling"), "use-optionals-for-ungrouped-worlds")
		move(shares, "use_optionals", section("share-handling"), "active-optional-shares")
		if len(shares) == 0 {
			delete(raw, "shares")
		}
	}
}

// activeOptionalShares answers the optional sharables config.yml turns on.
func (m *Module) activeOptionalShares() *Shares {
	sh, _ := m.Config.Custom("share-handling.active-optional-shares").(*Shares)
	if sh == nil {
		return &Shares{}
	}
	return sh
}
