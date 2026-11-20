package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// ConfigVersion is the config.yml version this port writes.
const ConfigVersion = 5.2

var (
	eventPriorities = []string{"lowest", "low", "normal", "high", "highest", "monitor"}
	passengerModes  = []string{"default", "dismount_passengers", "dismount_vehicle", "dismount_all",
		"retain_passengers", "retain_vehicle", "retain_all"}
	confirmModes = []string{"enable", "player_only", "disable_command_blocks", "disable_console", "disable"}
)

// configNodes is config.yml, in Multiverse-Core 5's order and with its
// comments. MV: CoreConfigNodes
func configNodes() []*Node {
	return append(worldConfigNodes(), otherConfigNodes()...)
}

// worldConfigNodes are config.yml's world and teleport sections.
func worldConfigNodes() []*Node {
	return []*Node{
		{Path: "world", Kind: KindHeader, Comments: []string{
			"####################################################################################################",
			"#                                                                                                  #",
			"#                    █▀▄▀█ █░█ █░░ ▀█▀ █ █░█ █▀▀ █▀█ █▀ █▀▀   █▀▀ █▀█ █▀█ █▀▀                      #",
			"#                    █░▀░█ █▄█ █▄▄ ░█░ █ ▀▄▀ ██▄ █▀▄ ▄█ ██▄   █▄▄ █▄█ █▀▄ ██▄                      #",
			"#                                                                                                  #",
			"#                                                                                                  #",
			"#    WIKI:        https://mvplugins.org/core/                                                      #",
			"#    DISCORD:     https://discord.gg/NZtfKky                                                       #",
			"#    BUG REPORTS: https://github.com/Multiverse/Multiverse-Core/issues                             #",
			"#    DONATE:      https://github.com/sponsors/Multiverse                                           #",
			"#                                                                                                  #",
			"#                                                                                                  #",
			"#    New options are added to this file automatically. If you manually made changes                #",
			"#    to this file while your server is running, please run `/mv reload` command.                   #",
			"#                                                                                                  #",
			"####################################################################################################",
			"",
			"",
		}},
		{Path: "world.auto-import-default-worlds", Name: "auto-import-default-worlds", Kind: KindBool, Default: true, Comments: []string{
			"When enabled, Multiverse will automatically import default worlds defined in the server.properties",
			"`level-name` property when the Multiverse is enabled or reloaded. This will include the nether and ",
			"end if the server created them.",
		}},
		{Path: "world.auto-import-3rd-party-worlds", Name: "auto-import-3rd-party-worlds", Kind: KindBool, Default: true, Comments: []string{
			"",
			"When enabled, Multiverse will import all other worlds created by other plugins when Multiverse starts",
			"or when Multiverse is reloaded.",
		}},
		{Path: "world.enforce-access", Name: "enforce-access", Kind: KindBool, Default: false, Comments: []string{
			"",
			"This setting will prevent players from entering worlds they don't have access to.",
			"If this is set to false, players will be able to enter any world they want.",
			"If this is set to true, players will only be able to enter worlds they have",
			"the `mv.access.<worldname>` permission.",
		}},
		{Path: "world.enforce-gamemode", Name: "enforce-gamemode", Kind: KindBool, Default: true, Comments: []string{
			"",
			"Sets whether Multiverse will should enforce gamemode on world change.",
			"If enabled, players will be forced into the gamemode of the world they are entering, unless they have",
			"the `mv.bypass.gamemode.<worldname>` permission.",
		}},
		{Path: "world.enforce-flight", Name: "enforce-flight", Kind: KindBool, Default: true, Comments: []string{
			"",
			"Sets whether Multiverse will should globally enforce flight ability on worlds.",
			"Disable this if you want another plugin to handle player's flight ability.",
			"Disabling this will make the world property `allow-flight` have no effect.",
		}},
		{Path: "world.gamemode-and-flight-enforce-delay", Name: "gamemode-and-flight-enforce-delay", Kind: KindInt, Default: 1, Comments: []string{
			"",
			"Sets the delay in ticks before Multiverse enforces gamemode and flight ability on world change.",
			"Increase this value if you are experiencing issues with other plugins overriding gamemode or flight ability.",
			"Or set to 0 to enforce immediately during world change event.",
		}},
		{Path: "world.apply-entity-spawn-rate", Name: "apply-entity-spawn-rate", Kind: KindBool, Default: true, Comments: []string{
			"",
			"Sets whether Multiverse will apply the world's entity `tick-rate` config in worlds.yml.",
			"If disabled, the `tick-rate` config in worlds.yml will be ignored.",
			"Disable this if you want paper-world.yml or another plugin to handle entity spawn rate per world.",
			"!!!NOTE: This will only apply after a server restart!",
		}},
		{Path: "world.apply-entity-spawn-limit", Name: "apply-entity-spawn-limit", Kind: KindBool, Default: true, Comments: []string{
			"",
			"Sets whether Multiverse will apply the world's entity `spawn-limit` config when a world is loaded.",
			"If disabled, the `spawn-limit` config in worlds.yml will be ignored.",
			"Disable this if you want paper-world.yml or another plugin to handle entity limits per world.",
			"!!!NOTE: This will only apply after a server restart!",
		}},
		{Path: "world.auto-purge-entities", Name: "auto-purge-entities", Kind: KindBool, Default: false, Comments: []string{
			"",
			"Sets whether Multiverse will purge entities on world load based world's entity spawn config.",
		}},
		{Path: "world.world-name-format", Kind: KindHeader, Comments: []string{
			"",
			"Format for world names for multiverse to automatically detect a world group consist of overworld, nether and end.",
			"This is used default-respawn-in-overworld and potentially other features.",
		}},
		{Path: "world.world-name-format.nether", Name: "nether-world-name-format", Kind: KindString, Default: "%overworld%_nether"},
		{Path: "world.world-name-format.end", Name: "end-world-name-format", Kind: KindString, Default: "%overworld%_the_end"},
		{Path: "world.warn-alias-conflicts", Name: "warn-alias-conflicts", Kind: KindBool, Default: true, Comments: []string{
			"",
			"Sets whether Multiverse will warn about alias duplicates when adding worlds or modifying aliases.",
			"Although not enforced, it is highly recommended to not have multiple worlds with the same alias or",
			"alias matching another world's name as it can cause confusion during world listing and selection in commands.",
		}},
		{Path: "teleport", Kind: KindHeader, Comments: []string{
			"",
			"",
		}},
		{Path: "teleport.use-finer-teleport-permissions", Name: "use-finer-teleport-permissions", Kind: KindBool, Default: true, Comments: []string{
			"Sets whether Multiverse will use more fine-grained teleport permissions.",
			"----",
			"New finer teleport permissions for /mvtp and /mvspawn commands:",
			"  - For specific teleport types: `multiverse.teleport.<self|other>.<type>.<target>`",
			"  - For specific world spawn: `multiverse.core.spawn.<self|other>.<worldname>`",
			"For example, if `multiverse.teleport.self.w.world2` is set, Multiverse will only allow the player to teleport to the world2.",
			"----",
			"Legacy permissions will be used if this is set to false:",
			"  - For teleport destinations: `multiverse.teleport.<self|other>.<type>`",
			"  - For spawn: `multiverse.core.spawn.<self|other>`",
		}},
		{Path: "teleport.passenger-mode", Name: "passenger-mode", Kind: KindEnum, Enum: passengerModes, Default: "default", Comments: []string{
			"",
			"Configures how passengers and vehicles are handled when an entity is teleported. This config is",
			"only applied to /mvtp, join-destination and first-spawn-override. It will not touch any other",
			"teleport by other plugins or commands.",
			"========= Available options: =========",
			"  default: Server will handle passengers and vehicles. On papermc 1.21.10+, all will be retained by default.",
			"                                                       On older versions, all will be dismounted if world changes.",
			"  dismount_passengers: Passengers will be removed from the parent entity before the teleport.",
			"  dismount_vehicle: Vehicle will be removed and from the parent entity before the teleport.",
			"  dismount_all: All passengers and vehicles will be removed from the parent entity before the teleport.",
			"  retain_passengers: Passengers will teleport together with the parent entity.",
			"  retain_vehicle: Vehicles will teleport together with the parent entity.",
			"  retain_all: All passengers and vehicles will teleport together with the parent entity.",
		}},
		{Path: "teleport.concurrent-teleport-limit", Name: "concurrent-teleport-limit", Kind: KindInt, Default: 50, Comments: []string{
			"",
			"Sets the maximum number of players allowed to be teleported at once with `/mv teleport` command",
		}},
		{Path: "teleport.teleport-intercept", Name: "teleport-intercept", Kind: KindBool, Default: true, Comments: []string{
			"",
			"If this is set to true, Multiverse will enforce access permissions for all teleportation,",
			"including teleportation from other plugins. You should not disable this unless you are facing",
			"conflict with another plugin handling teleportation.",
		}},
		{Path: "teleport.safe-location-horizontal-search-radius", Name: "safe-location-horizontal-search-radius", Kind: KindInt, Default: 3, Comments: []string{
			"",
			"Sets the horizontal (x and z-axis) search radius for finding a safe location to teleport to.",
			"Increasing this value will widen the search area at the cost of performance.",
			"To disable, set to 0.",
		}},
		{Path: "teleport.safe-location-vertical-search-radius", Name: "safe-location-vertical-search-radius", Kind: KindInt, Default: 3, Comments: []string{
			"",
			"Sets the vertical (y-axis) search radius for finding a safe location to teleport to.",
			"Increasing this value will widen the search area at the cost of performance.",
			"To disable, set to 0.",
		}},
	}
}

// otherConfigNodes are config.yml's spawn, portal, messaging, command,
// event-priority and misc sections.
func otherConfigNodes() []*Node {
	return []*Node{
		{Path: "spawn", Kind: KindHeader, Comments: []string{
			"",
			"",
		}},
		{Path: "spawn.first-spawn-override", Name: "first-spawn-override", Kind: KindBool, Default: false, Comments: []string{
			"Sets whether Multiverse will override the location where the player spawns when they join the server",
			"for the first time. For fixed spawn location on every login, see the `join-destination` option below.",
			"If disabled, it will default to server.properties settings.",
		}},
		{Path: "spawn.first-spawn-location", Name: "first-spawn-location", Kind: KindString, Default: "", Comments: []string{
			"",
			"Sets the world that Multiverse will use as the location for players that join the server for the first time.",
			"This only applies if first-spawn-override is set to true.",
		}},
		{Path: "spawn.enable-join-destination", Name: "enable-join-destination", Kind: KindBool, Default: false, Comments: []string{
			"",
			"Enables setting of a fixed location for players to spawn in when they join the server every time.",
			"See `join-destination` option below as well.",
		}},
		{Path: "spawn.join-destination", Name: "join-destination", Kind: KindString, Default: "", Comments: []string{
			"",
			"Sets the destination that Multiverse will use to spawn players on every login.",
			"Set the above enable-join-destination to false to disable",
		}},
		{Path: "spawn.default-respawn-in-overworld", Name: "default-respawn-in-overworld", Kind: KindBool, Default: true, Comments: []string{
			"",
			"This only applies if the `respawn-world` property is not set for the world that the player died in,",
			"and the player does not have bed or anchor set.",
			"----",
			"When this option is enabled, players will respawn in the overworld when dying in nether or end, mimicking the vanilla behavior.",
			"The automatic selection of overworld is determined by the `world-name-format` config section above.",
			"This option takes precedence over the `default-respawn-within-same-world` option.",
			"----",
			"Set this to false if you want another plugin to handle respawning or do not want this vanilla behavior.",
		}},
		{Path: "spawn.default-respawn-within-same-world", Name: "default-respawn-within-same-world", Kind: KindBool, Default: true, Comments: []string{
			"",
			"This only applies if the `respawn-world` property is not set for the world that the player died in,",
			"and the player does not have bed or anchor set.",
			"----",
			"When this option is enabled, players will respawn in the same world's that they died in.",
			"If the /spawnpoint is already within that world and `enforce-respawn-at-world-spawn` is disabled,",
			"Multiverse will use that spawn location, else it will use the world's spawn where the player died in.",
			"----",
			"You can set `respawn-world` property with the command: `/mv modify <worldname> set respawn-world <worldname>`",
			"You can reset `respawn-world` property with the command: `/mv modify <worldname> reset respawn-world`",
			"----",
			"Set this to false if you want another plugin to handle respawning.",
		}},
		{Path: "spawn.enforce-respawn-at-world-spawn", Name: "enforce-respawn-at-world-spawn", Kind: KindBool, Default: true, Comments: []string{
			"",
			"When this option is enabled, players will always respawn at the world's spawn location of calculated respawn world,",
			"unless bed or anchor is set and `bed-respawn` or `anchor-spawn` is enabled respectively.",
			"----",
			"Set this to false if you want to use a custom spawn location such as /spawnpoint instead of the world's spawn location.",
		}},
		{Path: "portal", Kind: KindHeader, Comments: []string{
			"",
			"",
		}},
		{Path: "portal.use-custom-portal-search", Name: "use-custom-portal-search", Kind: KindBool, Default: false, Comments: []string{
			"This config option defines whether or not Multiverse should interfere with's Bukkit's default portal search radius.",
			"Setting it to false would mean you want to simply let Bukkit decides the search radius itself.",
		}},
		{Path: "portal.custom-portal-search-radius", Name: "custom-portal-search-radius", Kind: KindInt, Default: 128, Comments: []string{
			"",
			"This config option defines the search radius Multiverse should use when searching for a portal.",
			"This only applies if use-custom-portal-search is set to true.",
		}},
		{Path: "messaging", Kind: KindHeader, Comments: []string{
			"",
			"",
		}},
		{Path: "messaging.enable-chat-prefix", Name: "enable-chat-prefix", Kind: KindBool, Default: false, Comments: []string{
			"This config option defines whether or not Multiverse should prefix the chat with the world name.",
			"Ensure this is false if you want another plugin to handle chat formatting.",
		}},
		{Path: "messaging.chat-prefix-format", Name: "chat-prefix-format", Kind: KindString, Default: "[%world%]%chat%", Comments: []string{
			"",
			"This config option defines the format Multiverse should use when prefixing the chat with the world name.",
			"This only applies if enable-chat-prefix is set to true.",
		}},
		{Path: "messaging.register-papi-hook", Name: "register-papi-hook", Kind: KindBool, Default: true, Comments: []string{
			"",
			"This config option defines whether or not Multiverse should register the PlaceholderAPI hook.",
			"This only applies if PlaceholderAPI is installed.",
		}},
		{Path: "messaging.warn-invalid-papi-format", Name: "warn-invalid-papi-format", Kind: KindBool, Default: true, Comments: []string{
			"",
			"This config option defines whether or not Multiverse should warn about invalid PlaceholderAPI formats.",
			"Only applies if PlaceholderAPI is installed, and only to multiverse placeholders.",
		}},
		{Path: "messaging.invalid-papi-format-returns-blank", Name: "invalid-papi-format-returns-blank", Kind: KindBool, Default: false, Comments: []string{
			"",
			"By default, invalid formats will return the original placeholder string.",
			"This config option defines whether or not Multiverse should show invalid PlaceholderAPI formats as blank.",
			"Only applies if PlaceholderAPI is installed, and only to multiverse placeholders.",
		}},
		{Path: "messaging.default-locale", Name: "default-locale", Kind: KindString, Default: "en", Comments: []string{
			"",
			"This config option defines the default language Multiverse should use.",
		}},
		{Path: "messaging.per-player-locale", Name: "per-player-locale", Kind: KindBool, Default: true, Comments: []string{
			"",
			"This config option defines if Multiverse should use the player's language based on their client's language.",
			"If the player's language does not have a translation, it will use the default language set above instead.",
		}},
		{Path: "command", Kind: KindHeader, Comments: []string{
			"",
			"",
		}},
		{Path: "command.resolve-alias-name", Name: "resolve-alias-name", Kind: KindBool, Default: true, Comments: []string{
			"If this is set to true, Multiverse will resolve world based on their alias names for commands and destinations.",
			"Normal world names will still be accepted.",
			"In the event you have multiple worlds with the same alias name, the first world found will be used.",
		}},
		{Path: "command.simplified-destination-tab-completion", Name: "simplified-destination-tab-completion", Kind: KindBool, Default: false, Comments: []string{
			"",
			"If this is set to true, Multiverse will only suggest simple destination formats in tab completion.",
			"This means only world names will be suggested without other destination type such as `e:worldname:x,y,z` or `p:playername`.",
			"Note: This DOES NOT prevent players from using the destinations, as that is controlled by permissions.",
		}},
		{Path: "command.confirm-mode", Name: "confirm-mode", Kind: KindEnum, Enum: confirmModes, Default: "enable", Comments: []string{
			"",
			"This config option defines whether `/mv confirm` is needed before running a DANGEROUS action.",
			"  enable: `/mv confirm` is required.",
			"  player_only: `/mv confirm` only required when running command as a player.",
			"  disable_command_blocks: `/mv confirm` not required for command blocks.",
			"  disable_console: `/mv confirm` not required for the console.",
			"  disable: `/mv confirm` is not required.",
		}},
		{Path: "command.use-confirm-otp", Name: "use-confirm-otp", Kind: KindBool, Default: true, Comments: []string{
			"",
			"If this is set to true, `/mv confirm` will include a 3 digit random number that must be entered to confirm the command.",
			"For example: `/mv confirm 726`",
		}},
		{Path: "command.confirm-timeout", Name: "confirm-timeout", Kind: KindInt, Default: 30, Comments: []string{
			"",
			"The amount of time in seconds before `/mv confirm` times out",
		}},
		{Path: "command.show-legacy-aliases", Name: "show-legacy-aliases", Kind: KindBool, Default: false, Comments: []string{
			"",
			"If this is set to true, legacy aliases will be shown in tab completion.",
			"These are old mv4 aliases such as `/mvclone` in addition to `/mv clone` which crowds the tab completion.",
			"!!!NOTE: This will only apply after a server restart!",
		}},
		{Path: "event-priority", Kind: KindHeader, Comments: []string{
			"",
			"",
		}},
		{Path: "event-priority.player-portal", Name: "event-priority-player-portal", Kind: KindEnum, Enum: eventPriorities, Default: "high", Comments: []string{
			"The follow configuration changes the bukkit's EventPriority for certain events.",
			"Only ever change this if you need multiverse's events outcomes to override another plugin, or if",
			"you want another plugin's outcome to override multiverse's.",
			"----",
			"!!!NOTE: This will only apply after a server restart!",
			"",
			"This config option defines the priority for the PlayerPortalEvent.",
		}},
		{Path: "event-priority.player-respawn", Name: "event-priority-player-respawn", Kind: KindEnum, Enum: eventPriorities, Default: "low", Comments: []string{
			"",
			"This config option defines the priority for the PlayerRespawnEvent.",
		}},
		{Path: "event-priority.player-spawn-location", Name: "event-priority-player-spawn-location", Kind: KindEnum, Enum: eventPriorities, Default: "normal", Comments: []string{
			"",
			"This config option defines the priority for the PlayerSpawnLocationEvent.",
		}},
		{Path: "event-priority.player-teleport", Name: "event-priority-player-teleport", Kind: KindEnum, Enum: eventPriorities, Default: "highest", Comments: []string{
			"",
			"This config option defines the priority for the PlayerTeleportEvent.",
		}},
		{Path: "event-priority.player-changed-world", Name: "event-priority-player-changed-world", Kind: KindEnum, Enum: eventPriorities, Default: "normal", Comments: []string{
			"",
			"This config option defines the priority for the PlayerChangedWorldEvent.",
		}},
		{Path: "misc", Kind: KindHeader, Comments: []string{
			"",
			"",
		}},
		{Path: "misc.bukkit-yml-path", Name: "bukkit-yml-path", Kind: KindString, Default: "bukkit.yml", Comments: []string{
			"Change this if you use a custom path for the bukkit.yml file with `--bukkit-settings` startup flag.",
			"Note: this config option needs a server restart to take effect.",
		}},
		{Path: "misc.server-properties-path", Name: "server-properties-path", Kind: KindString, Default: "server.properties", Comments: []string{
			"",
			"Change this if you use a custom path for the server.properties file with `--config` startup flag.",
			"Note: this config option needs a server restart to take effect.",
		}},
		{Path: "misc.auto-detect-generator-plugins", Name: "auto-detect-generator-plugins", Kind: KindBool, Default: true, Comments: []string{
			"",
			"When enabled, Multiverse will attempt to automatically detect world generator plugins installed on your server.",
			"This option only affects tab-completion within `/mv create` command and output of `/mv generators` command.",
			"Disabling this will not have any affect on the usages of world generator plugins itself.",
		}},
		{Path: "misc.global-debug", Name: "global-debug", Kind: KindInt, Default: 0, Comments: []string{
			"",
			"This is our debug flag to help identify issues with Multiverse.",
			"If you are having issues with Multiverse, please set this to 3 and then post your log to pastebin.com",
			"Otherwise, there's no need to touch this. If not instructed by a wiki page or developer.",
			"  0 = Off, No debug messages",
			"  1 = fine",
			"  2 = finer",
			"  3 = finest",
		}},
		{Path: "misc.debug-permissions", Name: "debug-permissions", Kind: KindBool, Default: false, Comments: []string{
			"",
			"Sets whether console will log every permission check done by all multiverse plugins.",
			"This will only work if the above 'global-debug' is set to 1 or more.",
		}},
		{Path: "misc.silent-start", Name: "silent-start", Kind: KindBool, Default: false, Comments: []string{
			"",
			"If true, the startup console messages will no longer show.",
		}},
		{Path: "misc.show-donation-message", Name: "show-donation-message", Kind: KindBool, Default: true, Comments: []string{
			"",
			"If you don't want to donate, you can set this to false and Multiverse will stop nagging you.",
			"But if you want to donate, here is our Github Sponsors page: https://github.com/sponsors/Multiverse",
		}},
		{Path: "version", Kind: KindDouble, Default: 0.0, Hidden: true, Comments: []string{
			"",
			"",
			"This just signifies the version number so we can see what version of config you have.",
			"NEVER TOUCH THIS VALUE",
		}},
	}
}

// Config is config.yml. MV: CoreConfig
type Config struct {
	*Values
	path    string
	migrate func(map[string]any)
	version float64
}

func newConfig(dir string) *Config {
	c := NewConfigFile(filepath.Join(dir, "config.yml"), configNodes(), migrateConfig, ConfigVersion)
	c.tune()
	return c
}

// NewConfigFile answers a config file of nodes at path, read and written as
// config.yml is: a module's own config.yml. migrate, when set, moves an
// older file's keys before they are read; version is written at "version".
func NewConfigFile(path string, nodes []*Node, migrate func(map[string]any), version float64) *Config {
	return &Config{Values: NewValues(NewNodes(nodes...)), path: path, migrate: migrate, version: version}
}

// tune adds the validators and suggestions config.yml's nodes have beyond
// their kind's.
func (c *Config) tune() {
	find := func(path string) *Node { n, _ := c.Nodes().byName[path]; return n }
	find("portal.custom-portal-search-radius").Validate = func(v any) error {
		if v.(int) < 0 {
			return MsgError(M("mv-core.core.config.customportalsearchradius.nonnegative"))
		}
		return nil
	}
	find("command.confirm-timeout").Validate = func(v any) error {
		if v.(int) <= 0 {
			return MsgError(M("mv-core.core.config.confirmtimeout.positive"))
		}
		return nil
	}
	debug := find("misc.global-debug")
	debug.Suggest = func(string) []string { return []string{"0", "1", "2", "3"} }
	debug.Validate = func(v any) error {
		if i := v.(int); i < 0 || i > 3 {
			return MsgError(M("mv-core.core.config.debuglevel.range"))
		}
		return nil
	}
}

// Load reads config.yml, migrating an older one, and writes it back with
// every option. A missing file is created with the defaults.
func (c *Config) Load() error {
	data, err := os.ReadFile(c.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// MV: CoreConfig.migrateFromOldConfigFile
	data = bytes.ReplaceAll(data, []byte("==: com.onarandombox.MultiverseCore.MultiverseCoreConfiguration"), nil)
	m := map[string]any{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("config.yml: %w", err)
	}
	if m == nil {
		m = map[string]any{}
	}
	if c.migrate != nil {
		c.migrate(m)
	}
	c.Values.Load(m)
	c.setRaw("version", c.version)
	return c.Save()
}

// Save writes config.yml with its comments.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(c.path, c.render(), 0o644)
}

// render writes the nodes in order: each comment line as "# text" (an empty
// one as a blank line), sections indented two spaces.
func (c *Config) render() []byte {
	var b strings.Builder
	open := []string{}
	for _, n := range c.Nodes().All() {
		parts := strings.Split(n.Path, ".")
		section := parts
		if n.Kind != KindHeader {
			section = parts[:len(parts)-1]
		}
		common := 0
		for common < len(open) && common < len(section) && open[common] == section[common] {
			common++
		}
		open = open[:common]
		writeComments := func(depth int) {
			for _, line := range n.Comments {
				if line == "" {
					b.WriteString("\n")
					continue
				}
				if !strings.HasPrefix(line, "#") {
					line = "# " + line
				}
				b.WriteString(strings.Repeat("  ", depth) + line + "\n")
			}
		}
		for len(open) < len(section) {
			depth := len(open)
			if n.Kind == KindHeader && depth == len(section)-1 {
				writeComments(depth)
			}
			b.WriteString(strings.Repeat("  ", depth) + section[depth] + ":\n")
			open = append(open, section[depth])
		}
		if n.Kind == KindHeader {
			continue
		}
		writeComments(len(section))
		indent := strings.Repeat("  ", len(section))
		if list, ok := encodeValue(n, c.Get(n)).([]string); ok && len(list) > 0 {
			b.WriteString(indent + parts[len(parts)-1] + ":\n")
			for _, item := range list {
				b.WriteString(indent + "  - " + yamlScalar(item) + "\n")
			}
			continue
		}
		b.WriteString(indent + parts[len(parts)-1] + ": " + yamlScalar(encodeValue(n, c.Get(n))) + "\n")
	}
	if len(c.extra) > 0 {
		keys := make([]string, 0, len(c.extra))
		for k := range c.extra {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		extra := map[string]any{}
		for _, k := range keys {
			setPath(extra, k, c.extra[k])
		}
		out, err := yaml.Marshal(extra)
		if err == nil {
			b.WriteString("\n")
			b.Write(out)
		}
	}
	return []byte(b.String())
}

// yamlScalar writes one value as YAML, quoting a string only when it must.
func yamlScalar(v any) string {
	if f, ok := v.(float64); ok {
		return JavaDouble(f)
	}
	if s, ok := v.(string); ok && s == "" {
		return "''"
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return strings.TrimSuffix(string(out), "\n")
}

func setPath(m map[string]any, path string, v any) {
	parts := strings.Split(path, ".")
	for _, p := range parts[:len(parts)-1] {
		sub, ok := m[p].(map[string]any)
		if !ok {
			sub = map[string]any{}
			m[p] = sub
		}
		m = sub
	}
	m[parts[len(parts)-1]] = v
}

func deletePath(m map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	for _, p := range parts[:len(parts)-1] {
		sub, ok := m[p].(map[string]any)
		if !ok {
			return nil, false
		}
		m = sub
	}
	v, ok := m[parts[len(parts)-1]]
	delete(m, parts[len(parts)-1])
	return v, ok
}

// migrateConfig moves a Multiverse-Core 4 config.yml's keys to version 5's.
// MV: CoreConfig's ConfigMigrator
func migrateConfig(m map[string]any) {
	version, _ := lookupPath(m, "version")
	v, _ := version.(float64)
	if i, ok := version.(int); ok {
		v = float64(i)
	}
	move := func(from, to string, conv func(any) any) {
		if val, ok := deletePath(m, from); ok {
			if conv != nil {
				val = conv(val)
			}
			setPath(m, to, val)
		}
	}
	toBool := func(x any) any { b, _ := ParseString(&Node{Kind: KindBool}, fmt.Sprint(x)); return b }
	invert := func(x any) any { b, _ := toBool(x).(bool); return !b }
	toInt := func(x any) any { i, _ := ParseString(&Node{Kind: KindInt}, fmt.Sprint(x)); return i }
	if v < 5.0 {
		old := "multiverse-configuration."
		move(old+"enforceaccess", "world.enforce-access", toBool)
		move(old+"prefixchat", "messaging.enable-chat-prefix", toBool)
		move(old+"prefixchatformat", "messaging.chat-prefix-format", nil)
		move(old+"teleportintercept", "world.teleport-intercept", toBool)
		move(old+"firstspawnoverride", "spawn.first-spawn-override", toBool)
		move(old+"displaypermerrors", "misc.debug-permissions", nil)
		move(old+"globaldebug", "misc.global-debug", toInt)
		move(old+"silentstart", "misc.silent-start", toBool)
		move(old+"firstspawnworld", "spawn.first-spawn-location", nil)
		move(old+"defaultportalsearch", "portal.use-custom-portal-search", invert)
		move(old+"portalsearchradius", "portal.custom-portal-search-radius", toInt)
		move(old+"autopurge", "world.auto-purge-entities", toBool)
		move(old+"idonotwanttodonate", "misc.show-donation-message", invert)
		setPath(m, "command.show-legacy-aliases", true)
		delete(m, "multiverse-configuration")
	}
	if v < 5.1 {
		move("world.teleport-intercept", "teleport.teleport-intercept", nil)
		move("world.resolve-alias-name", "command.resolve-alias-name", nil)
	}
	if v < 5.2 {
		move("spawn.default-respawn-to-world-spawn", "world.enforce-respawn-at-world-spawn", nil)
	}
}
