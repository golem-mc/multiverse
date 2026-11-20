package portals

import (
	"strings"

	"github.com/golem-mc/golem/registry"
	"github.com/golem-mc/multiverse/core"
)

// configVersion is the config.yml version this port writes.
const configVersion = 5.1

// configNodes is Multiverse-Portals' config.yml, in its order and with its
// comments. MV: PortalsConfigNodes
func configNodes() []*core.Node {
	return []*core.Node{
		{Path: "portal-creation", Kind: core.KindHeader, Comments: []string{
			"#-------------------------------------------------------------------------------------------------------#",
			"#                                                                                                       #",
			"#          __  __ _   _ _  _____ _____   _____ ___  ___ ___   ___  ___  ___ _____ _   _    ___          #",
			"#         |  \\/  | | | | ||_   _|_ _\\ \\ / / __| _ \\/ __| __| | _ \\/ _ \\| _ \\_   _/_\\ | |  / __|         #",
			"#         | |\\/| | |_| | |__| |  | | \\ V /| _||   /\\__ \\ _|  |  _/ (_) |   / | |/ _ \\| |__\\__ \\         #",
			"#         |_|  |_|\\___/|____|_| |___| \\_/ |___|_|_\\|___/___| |_|  \\___/|_|_\\ |_/_/ \\_\\____|___/         #",
			"#                                                                                                       #",
			"#                                                                                                       #",
			"#               WIKI:        https://mvplugins.org/portals/                                             #",
			"#               DISCORD:     https://discord.gg/NZtfKky                                                 #",
			"#               BUG REPORTS: https://github.com/Multiverse/Multiverse-Portals/issues                    #",
			"#               DONATE:      https://github.com/sponsors/Multiverse                                     #",
			"#                                                                                                       #",
			"#                                                                                                       #",
			"#           New options are added to this file automatically. If you manually made changes              #",
			"#           to this file while your server is running, please run `/mvp reload` command.                #",
			"#                                                                                                       #",
			"#-------------------------------------------------------------------------------------------------------#",
			"",
			"",
		}},
		{Path: "portal-creation.wand-material", Name: "wand-material", Kind: core.KindString, Default: "wooden_pickaxe",
			Parse: parseItemName, Comments: []string{
				"The item used to select a region to create a mvportal. Run `/mv wand` to start selection.",
				"This will be ignore if worldedit is used for selection instead.",
			}},
		{Path: "portal-creation.bucket-filling", Name: "bucket-filling", Kind: core.KindBool, Default: true, Comments: []string{
			"",
			"If enabled, water and lava bucket can be used to fill a mvportal.",
		}},
		{Path: "portal-creation.frame-materials", Name: "frame-materials", Kind: core.KindList, Default: []string{},
			Parse: func(s string) (any, error) {
				name, err := parseBlockName(s)
				if err != nil {
					return nil, err
				}
				return []string{name.(string)}, nil
			}, Comments: []string{
				"",
				"The allowed materials used to create the frame of the portal.",
				"If the list is empty, all materials are allowed.",
			}},
		{Path: "portal-creation.clear-on-remove", Name: "clear-on-remove", Aliases: []string{"clearonremove"},
			Kind: core.KindBool, Default: false, Comments: []string{
				"",
				"If enabled, nether/end fills will be cleared when the portal is removed.",
				"This keeps us from leaving behind portal blocks (which would take an unsuspecting player to the nether when trying to use the mvportal)",
			}},
		{Path: "portal-usage", Kind: core.KindHeader, Comments: []string{"", ""}},
		{Path: "portal-usage.enforce-portal-access", Name: "enforce-portal-access", Aliases: []string{"enforceportalaccess"},
			Kind: core.KindBool, Default: true, Comments: []string{
				"If enabled, players will not be able to teleport to mvportals they do not have access to.",
				"The permission node is: `multiverse.portal.access.<portal-name>`",
			}},
		{Path: "portal-usage.send-no-permission-messages", Name: "send-no-permission-messages", Kind: core.KindBool, Default: true,
			Comments: []string{
				"",
				"This is a global toggle to disable the sending of no permission messages for all portals.",
				"This is only applicable if `enforce-portal-access` is enabled above, else permission checks will not happen anyways.",
				"If you are looking to customize/disable specific portal's message, use each portal's `no-permission-message` property.",
			}},
		{Path: "portal-usage.portal-cooldown", Name: "portal-cooldown", Aliases: []string{"portalcooldown"},
			Kind: core.KindInt, Default: 1000, Comments: []string{
				"",
				"The time (in milliseconds) a player must wait between using a mvportal.",
			}},
		{Path: "portal-usage.portals-default-to-nether", Name: "portals-default-to-nether", Aliases: []string{"portalsdefaulttonether"},
			Kind: core.KindBool, Default: false, Comments: []string{
				"",
				"If enabled, when a mvportal with nether fill that is unusable, either due to invalid destination or lack of permissions,",
				"will fallback and behave as a normal nether portal teleporting between the nether world.",
			}},
		{Path: "portal-usage.nether-animation", Name: "nether-animation", Kind: core.KindBool, Default: true, Comments: []string{
			"",
			"If enabled, the nether blobbing animation will be played before the player is teleported.",
			"Note: This does not work if the player is in creative mode due to server software limitations.",
		}},
		{Path: "portal-usage.teleport-vehicles", Name: "teleport-vehicles", Aliases: []string{"teleportvehicles"},
			Kind: core.KindBool, Default: false, Comments: []string{
				"",
				"If enabled, mvportals will teleport all vehicles along with its passengers when the vehicle enters the portal.",
				"Vehicles are usually boats and minecarts.",
				"(NOTE: please turn off the server before changing this config option)",
			}},
		{Path: "portal-usage.teleport-entities", Name: "teleport-entities", Kind: core.KindBool, Default: false, Comments: []string{
			"",
			"If enabled, all living entities can use the portal if property `teleport-non-players` is true.",
			"There may be some performance overhead if your server has a large amount of entities moving around.",
			"(NOTE: please turn off the server before changing this config option)",
		}},
		{Path: "portal-usage.use-on-move", Name: "use-on-move", Aliases: []string{"useonmove"},
			Kind: core.KindBool, Default: true, Comments: []string{
				"",
				"If enabled, player movement will be tracked to determine if the player/entity has entered a portal.",
				"Disabling this will cause mvportals without nether or end fill to not work.",
				"Only disable this if all your portals have nether or end fill and want to slight enhance performance.",
				"(NOTE: please turn off the server before changing this config option)",
			}},
		{Path: "version", Kind: core.KindDouble, Default: 0.0, Hidden: true, Comments: []string{
			"",
			"",
			"This just signifies the version number so we can see what version of config you have.",
			"NEVER TOUCH THIS VALUE",
		}},
	}
}

// migrateConfig moves a Multiverse-Portals 4 config.yml's keys to version
// 5's. An empty file is new and needs none. MV: PortalsConfig's ConfigMigrator
func migrateConfig(m map[string]any) {
	if len(m) == 0 {
		return
	}
	v := versionOf(m)
	if v < 5.0 {
		for _, mv := range [][2]string{
			{"wand", "portal-creation.wand-material"},
			// Multiverse moves "useronmove", a misspelling of the key
			// Multiverse-Portals 4 wrote.
			{"useronmove", "portal-usage.use-on-move"},
			{"useonmove", "portal-usage.use-on-move"},
			{"bucketfilling", "portal-creation.bucket-filling"},
			{"portalsdefaulttonether", "portal-usage.portals-default-to-nether"},
			{"enforceportalaccess", "portal-usage.enforce-portal-access"},
			{"portalcooldown", "portal-usage.portal-cooldown"},
			{"clearonremove", "portal-creation.clear-on-remove"},
			{"teleportvehicles", "portal-usage.teleport-vehicles"},
			{"netheranimation", "portal-usage.nether-animation"},
			{"framematerials", "portal-creation.frame-materials"},
		} {
			movePath(m, mv[0], mv[1])
		}
	}
	if v < 5.1 {
		setPath(m, "portal-usage.send-no-permission-messages", false)
	}
}

// parseItemName reads a material that is an item, as config.yml keeps it:
// lowercase, without "minecraft:". MV: MaterialConverter.stringToMaterial
func parseItemName(s string) (any, error) {
	name := materialPath(s)
	if _, ok := registry.ItemByName("minecraft:" + name); !ok {
		return nil, core.MsgError(core.Lit("No enum constant " + strings.ToUpper(name)))
	}
	return name, nil
}

// parseBlockName reads a material that is a block.
func parseBlockName(s string) (any, error) {
	name := materialPath(s)
	if _, ok := registry.BlockByName("minecraft:" + name); !ok {
		return nil, core.MsgError(core.Lit("No enum constant " + strings.ToUpper(name)))
	}
	return name, nil
}

func materialPath(s string) string {
	return strings.TrimPrefix(strings.ToLower(s), "minecraft:")
}

// materialName writes a material as Bukkit's Material.toString does.
func materialName(path string) string { return strings.ToUpper(path) }

// wandItem answers the configured wand.
func (p *Portals) wandItem() registry.Item {
	it, _ := registry.ItemByName("minecraft:" + p.Config.String("portal-creation.wand-material"))
	return it
}
