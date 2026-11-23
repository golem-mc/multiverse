# Multiverse for golem

An unofficial port of [Multiverse](https://github.com/Multiverse) 5 to [golem](https://github.com/golem-mc/golem): manage several worlds, give them their own rules, inventories and portals, and link the nether and end between them. It is not made or endorsed by the Multiverse Team.

Commands, permissions and config files follow Multiverse 5. For rarely used corners, see the [Multiverse documentation](https://mvplugins.org/).

## Installing

```sh
golem-launcher plugin add github.com/golem-mc/multiverse
```

Optional modules (each needs the core above):

```sh
golem-launcher plugin add github.com/golem-mc/multiverse/inventories    # separate inventories and stats per world or group
golem-launcher plugin add github.com/golem-mc/multiverse/portals        # custom portals
golem-launcher plugin add github.com/golem-mc/multiverse/netherportals  # link nether and end portals between worlds
```

Entry fees can use [Vault](https://github.com/golem-mc/vault) money when it is installed; without it they take items.

## Getting started

All world commands start with `/mv` (`/mv help` lists them). Every command is also available as a legacy root such as `/mvcreate` or `/mvtp`.

```text
/mv create creative normal --seed 1234     create a world
/mv modify set gamemode creative creative  its players play in creative
/mv tp creative                            go there
/mv spawn                                  back to the world's spawn
/mv list                                   all worlds you may enter
/mv import mymap normal                    adopt a world folder you copied in
```

Then add what you need: a group of worlds sharing inventories (`/mvinv create-group`), a portal between two spots (`/mvp wand`, select two corners, `/mvp create hub w:creative`) or a nether link (`/mvnp link nether creative creative_nether`).

Destinations, used by `/mv tp`, portals and `join-destination`, are `w:<world>`, `e:<world>:<x>,<y>,<z>[:pitch:yaw]`, `pl:<player>`, `b:<player>` (their bed), `a:<anchor>`, `ca:...` (cannon), `p:<portal>`, `rp:<world>` (random portal) and `ll:<world>` (last location).

## Commands

### Worlds (`/mv`)

| Usage | What it does | Permission |
|---|---|---|
| `/mv create <name> <environment> [--seed --generator <gen[:id]> --world-type --no-structures]` | Create a world | `multiverse.core.create` |
| `/mv import <name> <environment> [--generator --no-adjust-spawn --biome]` | Import an existing world folder | `multiverse.core.import` |
| `/mv load <world>` / `/mv unload <world> [--remove-players [dest]]` | Load or unload a world | `multiverse.core.load` / `.unload` |
| `/mv remove <world>` | Forget a world, keep its files | `multiverse.core.remove` |
| `/mv delete <world>` | Remove and delete a world (asks to `/mv confirm`) | `multiverse.core.delete` |
| `/mv clone <world> <new> [--reset-world-config --reset-gamerules --reset-world-border]` | Copy a world | `multiverse.core.clone` |
| `/mv regen <world> [--seed [seed]]` | Regenerate a world (asks to confirm) | `multiverse.core.regen` |
| `/mv list [--filter --page --raw]` | List worlds | `multiverse.core.list.worlds` |
| `/mv info [world]` | Show a world's settings | `multiverse.core.info` |
| `/mv who [world]`, `/mv whoall` | Players in a world, or in all | `multiverse.core.list.who`, `.who.all` |
| `/mv coordinates` (`coord`) | Your position and world | `multiverse.core.coord` |
| `/mv modify [world] <set\|add\|remove\|reset> <property> [value]` | Change a world property (below) | `multiverse.core.modify` |
| `/mv meta info\|modify` | Per-world key/value data | `multiverse.core.meta.info`, `.modify` |
| `/mv entity-spawn-config info\|modify` | Per-world mob spawn rates and limits | `multiverse.core.entityspawnconfig.info`, `.modify` |
| `/mv gamerule set\|reset\|list` (`rule`) | Game rules of the world(s) | `multiverse.core.gamerule.set`, `.list` |
| `/mv worldborder add\|set\|get\|center\|damage\|warning ...` | World border of a world | `multiverse.core.worldborder` |
| `/mv purge-entities [world]`, `/mv purge-all-entities [world] [categories]` | Remove mobs by spawn category | `multiverse.core.purge`, `.purgeall` |
| `/mv generators list\|info` | Available world generators | `multiverse.core.generator` |

### Teleporting and anchors

| Usage | What it does | Permission |
|---|---|---|
| `/mv teleport [player] <destination> [--unsafe --silent]` (`tp`, `/mvtp`) | Teleport to any destination | `multiverse.teleport.self.<type>[.<target>]`, `.other.` for others |
| `/mv spawn [player]` | Go to the world's spawn | `multiverse.core.spawn.self`, `.other` |
| `/mv setspawn [world:x,y,z]` | Set the world spawn | `multiverse.core.spawn.set` |
| `/mv check <player> <destination>` | Whether the player may use a destination | `multiverse.core.check` |
| `/mv anchor set\|delete\|list` | Named locations (`a:<name>`) | `multiverse.core.anchor.create`, `.delete`, `.list` |

### Admin

| Usage | What it does | Permission |
|---|---|---|
| `/mv config <name> [value]` | Read or set a `config.yml` option | `multiverse.core.config` |
| `/mv reload` | Reload all Multiverse files | `multiverse.core.reload` |
| `/mv confirm [code]` | Confirm a destructive command | `multiverse.core.confirm` |
| `/mv debug <level>`, `/mv dumps [--upload pastesdev\|mclogs]` | Debugging and a support dump | `multiverse.core.debug`, `.dumps` |
| `/mv version`, `/mv help [filter] [page]` | Version and command list | `multiverse.core.version`, `.help` |

World properties for `/mv modify`: `alias`, `gamemode`, `difficulty`, `pvp`, `hunger`, `auto-heal`, `allow-flight`, `allow-weather`, `auto-load`, `hidden`, `keep-spawn-in-memory`, `adjust-spawn`, `bed-respawn`, `anchor-respawn`, `respawn-world`, `player-limit` (-1 is unlimited), `portal-form`, `scale`, `world-blacklist`, `allow-advancement-grant`, `entryfee-enabled`, `entryfee-amount` and `entryfee-currency`.

### Inventories (`/mvinv`)

| Usage | What it does | Permission |
|---|---|---|
| `list`, `info <world\|group>` | Show groups and what a world shares | `multiverse.inventories.list`, `.info` |
| `create-group <name> [worlds] [shares]`, `delete-group <group>`, `group` | Create, delete or edit a group (`group` asks step by step) | `.creategroup`, `.deletegroup`, `.group` |
| `add-worlds`, `remove-worlds`, `add-shares`, `remove-shares`, `add-disabled-shares`, `remove-disabled-shares` `<group> <list>` | Edit a group | `.addworlds`, `.removeworlds`, `.addshares`, ... |
| `toggle <share>` | Turn an optional share (`economy`, `last_location`) on or off | `.addshares` |
| `view <player> <world>`, `modify <player> <world>` | Open a player's stored inventory, read-only or editable | `.view`, `.view.modify` |
| `give <player> <world> [gamemode] <item> [amount]` | Give an item into a stored profile | `.give` |
| `bulkedit ...` | Clear, clone, delete or migrate many profiles | `.bulkedit` |
| `playerdata import <world>`, `migrate <MultiInv\|WorldInventories\|PerWorldInventory>` | Import old data | `.importplayerdata`, `.migrate` |
| `check-group-conflicts`, `cache stats\|invalidate`, `config <name> [value]`, `reload` | Maintenance | `.checkgroupconflict`, `.cache.*`, `.config`, `.reload` |

All nodes above start with `multiverse.inventories`.

### Portals (`/mvp`)

| Usage | What it does | Permission |
|---|---|---|
| `wand [enable\|disable\|toggle]` | Get the selection tool (a wooden pickaxe by default): left click the first corner, right click the second | `multiverse.portal.givewand` |
| `create <name> [destination]` | Create a portal in your selection | `multiverse.portal.create` |
| `select <portal>` | Select a portal's region again | `multiverse.portal.select` |
| `modify [portal] <property> <value>` | Change a portal (below) | `multiverse.portal.modify` |
| `list [filter/world] [page]`, `info [portal]` | List portals and show one | `multiverse.portal.list`, `.info` |
| `remove <portal>` | Delete a portal | `multiverse.portal.remove` |
| `config <property> [value]`, `debug [on\|off]` | Plugin settings and debugging | `multiverse.portal.config`, `.debug` |

Portal properties: `destination` (or `action-type` and `action`: a destination, a `command`, or a BungeeCord `server`), `safe-teleport`, `check-destination-safety`, `teleport-non-players`, `price` and `currency` (an entry fee), `no-permission-message`, `action-success-message`, `owner` and `location`. Fill a portal's frame with `nether_portal`, `water`, `lava` or similar blocks with a bucket or by `portal-creation.frame-materials`.

### Nether and End portals (`/mvnp`)

| Usage | What it does | Permission |
|---|---|---|
| `link <nether\|end> [fromWorld] <toWorld>` | Send a world's nether or end portals to another world | `multiverse.netherportals.link` |
| `unlink <nether\|end> [fromWorld]` | Remove the link | `multiverse.netherportals.unlink` |
| `list [nether\|end]` | Show links | `multiverse.netherportals.show` |

Worlds are linked automatically by name (`cat` and `cat_nether`, `cat_the_end`; see the prefixes and suffixes in the config), coordinates are scaled by each world's `scale`, and a player who cannot use the portal is bounced back.

## Permissions

Operators have every command node by default; give them to others with a permissions plugin. The nodes that matter on a server:

| Node | What it controls | Default |
|---|---|---|
| `multiverse.access.<world>` | Entering a world, when `enforce-access` is on | op |
| `multiverse.exempt.<world>` | Skip the world's entry fee | op |
| `multiverse.teleport.self.<type>[.<target>]`, `.other.` | `/mv tp` by destination type (`w`, `e`, `pl`, `b`, `a`, `p`, ...) and target, with `use-finer-teleport-permissions` | op |
| `multiverse.core.spawn.self`, `.other`, `.<world>` | `/mv spawn` | op |
| `mv.bypass.gamemode.<world>` | Keep your game mode in a world | nobody |
| `mv.bypass.playerlimit.<world>` | Join a full world | nobody |
| `mv.bypass.joinlocation` | Skip the join destination | nobody |
| `multiverse.portal.access.<portal>` | Use a portal, with `enforce-portal-access` | op |
| `multiverse.portal.fill.<portal>` | Fill a portal's frame | op |
| `multiverse.portal.exempt.<portal>` | Use a portal for free | nobody |
| `mvinv.bypass.*` | Bypass share handling, with `enable-bypass` | nobody |

Wildcards work: `multiverse.access.*`, `multiverse.core.*`, `multiverse.teleport.*`.

## Configuration

Files are in `plugins/multiverse-core/` (`config.yml`, `worlds.yml`, `anchors.yml`), `plugins/multiverse-inventories/` (`config.yml`, `groups.yml`, `players/`, `worlds/`, `groups/`), `plugins/multiverse-portals/` (`config.yml`, `portals.yml`) and `plugins/multiverse-netherportals/` (`config.yml`, `links.yml`). Each file explains its options in comments; `/mv reload` applies changes. Main options:

| Option | Default |
|---|---|
| core `world.enforce-access` | `false` |
| core `world.enforce-gamemode` / `enforce-flight` | `true` |
| core `world.auto-import-default-worlds` / `auto-import-3rd-party-worlds` | `true` |
| core `teleport.use-finer-teleport-permissions` | `true` |
| core `teleport.passenger-mode` | `default` |
| core `spawn.first-spawn-override` / `first-spawn-location` | `false` / empty |
| core `spawn.enable-join-destination` / `join-destination` | `false` / empty |
| core `spawn.default-respawn-in-overworld` / `default-respawn-within-same-world` | `true` |
| core `messaging.enable-chat-prefix` / `chat-prefix-format` | `false` / `[%world%]%chat%` |
| core `messaging.default-locale` / `per-player-locale` | `en` / `true` |
| core `command.confirm-mode` / `confirm-timeout` | `enable` / `30` |
| inventories `share-handling.enable-gamemode-share-handling` | `false` |
| inventories `share-handling.default-ungrouped-worlds` | `false` |
| inventories `share-handling.enable-bypass-permissions` | `false` |
| portals `portal-creation.wand-material` | `wooden_pickaxe` |
| portals `portal-usage.enforce-portal-access` | `true` |
| portals `portal-usage.portal-cooldown` (ms) | `1000` |
| portals `portal-usage.use-on-move` | `true` |
| netherportals `nether-suffix` / `end-suffix` | `_nether` / `_the_end` |
| netherportals `bounceback` | `true` |
| netherportals `teleport-entities` | `true` |

Change core and inventories options in game with `/mv config` and `/mvinv config`, and portals with `/mvp config`.

Groups in `groups.yml` list worlds and the data they share: `inventory`, `ender_chest`, `armor`, `off_hand`, `health`, `hunger`, `exp`, `potion_effects`, `bed_spawn`, `game_statistics` and, as optional shares, `economy` and `last_location`; `all` shares everything. The `advancements` and `recipes` shares are not switched between worlds.

## Migrating from Bukkit or Paper

Stop the old server, copy each `plugins/Multiverse-*/` folder's files (`config.yml`, `worlds.yml`, `groups.yml`, `portals.yml`, `links.yml`, `anchors.yml` and the player data folders) into the matching directory above and start golem. Multiverse 4 configs and player data are converted on first start. World folders keep working with `/mv import`.

Works with: [Vault](https://github.com/golem-mc/vault) (entry fees and per-world balances).

## Differences from Multiverse 5

- World names map to golem dimension keys: `world` is the vanilla overworld, `world_nether` and `world_the_end` its dimensions, and any other name becomes `minecraft:<name>`.
- `/mv delete` and `/mv regen` refuse the three vanilla dimensions.
- `/mv tp` accepts only the argument-less player selectors `@a`, `@p`, `@r`, `@s` and `@e`.
- `/mv gamerule` also accepts `keepInventory` and `keep_inventory`, and takes only `true` or `false` for a boolean rule.
- `--generate-bonus-chest` has no effect, game rules are server-wide rather than per world, and portals cannot select regions with WorldEdit (use `/mvp wand`).
- Multiverse-InventoriesImporter, PlaceholderAPI placeholders and LuckPerms world contexts are not available.

## License

BSD-3-Clause, like Multiverse. See [LICENSE](LICENSE).
