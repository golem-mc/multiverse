# Multiverse for golem

An unofficial port of [Multiverse](https://github.com/Multiverse) to
[golem](https://github.com/golem-mc/golem), the Minecraft: Java Edition server written in Go.
It is not made or endorsed by the Multiverse Team.

It keeps Multiverse-Core 5's commands, permission nodes, messages (in all of
Multiverse's languages) and configuration files, so an admin coming from
Bukkit or Paper finds the same `/mv` and the same `config.yml` and
`worlds.yml`.

## What's included

| Plugin | Package | What it does |
|---|---|---|
| Multiverse-Core | `github.com/golem-mc/multiverse` | `/mv` and its world manager |
| Multiverse-Inventories | `github.com/golem-mc/multiverse/inventories` | Separate inventories and stats per world, group and game mode |
| Multiverse-Portals | `github.com/golem-mc/multiverse/portals` | `/mvp`: portals that send whoever walks in to a destination |

Multiverse-Core:

- `config.yml` with every Multiverse-Core 5.8 option and its comments; a
  Multiverse 4 `config.yml` is migrated
- `worlds.yml` with every world property, read from a Bukkit server's file
  (worlds named by name or by namespaced key) and written back in the same
  form; a Multiverse 4 `worlds.yml` (kept as `worlds.yml.old.<time>`) and
  world sections of versions 1.0 to 1.2 are migrated
- the vanilla worlds are imported as `world`, `world_nether` and
  `world_the_end` (after `level-name`), worlds made by other plugins too;
  worlds with `auto-load` load on start
- `/mv list`, `/mv info`, `/mv config`, `/mv debug`, `/mv reload`,
  `/mv confirm`, `/mv help`, `/mv version`, with their flags (`--page`,
  `--filter`, `--raw`), tab completion and the Multiverse 4 aliases
  (`/mvlist`, `/mvinfo`, ...) when `show-legacy-aliases` is on
- world types (`normal`, `flat`, `large_biomes`, `amplified`), flat
  `--generator-settings`, single-biome worlds (`--biome`) and
  `--no-structures`; generators by plugin name (`-g Plugin[:id]`) from any
  golem plugin that publishes them (`plugin.GeneratorProvider`; it should
  load before `multiverse-core` so `auto-load` worlds find it)
- `/mv create`, `/mv import`, `/mv load`, `/mv unload`, `/mv remove`,
  `/mv delete`, `/mv clone` and `/mv regen`, with their flags (`--seed`,
  `--generator`, `--properties`, `--force-spawn-position`,
  `--remove-players [destination]`, `--no-save`, `--keep-files`, ...), the
  `/mv confirm` step for delete and regen, and tab completion of unknown
  world folders for import
- the world (`w:`) destination for `--remove-players`
- `/mv coordinates`, `/mv dumps` (`--upload pastesdev|mclogs`, with
  `MVDumpsDebugInfoEvent` for modules to add files),
  `/mv entity-spawn-config info|modify`, `/mv gamerule set|reset|list`,
  `/mv generators list|info`, `/mv meta info|modify`, `/mv purge-entities`,
  `/mv purge-all-entities [spawn-categories]`, `/mv who`, `/mv whoall` and
  `/mv worldborder add|center|damage amount|damage buffer|get|set|warning
  distance|warning time`, with their Multiverse 4 aliases (`/mvrule`,
  `/mvrules`, `/mvwho`, `/mvw`, `/mvwhoall`, `/mvgens`, `/mvco`, ...)
- `/mv teleport` (`/mvtp`), `/mv spawn`, `/mv setspawn`, `/mv check` and
  `/mv anchor set|delete|list` with `anchors.yml`; every destination type
  (`w:`, `e:`, `pl:`, `a:`, `b:`, `ca:`), the safe-location search,
  `--unsafe`, `--silent`, passenger modes, `use-finer-teleport-permissions`
  and `concurrent-teleport-limit`
- world entry checks on teleports and portals: `enforce-access`,
  `player-limit`, `world-blacklist` and entry fees in items or, with
  `@vault-economy`, through [golem-mc/vault](https://github.com/golem-mc/vault)'s
  `vault.Economy` from whichever economy plugin provides it (charged, or paid
  out when negative), with `teleport-intercept`; `join-destination` and
  `first-spawn-override` on join
- the permission nodes `multiverse.core.*`, and per world
  `multiverse.access.<world>`, `multiverse.exempt.<world>`,
  `mv.bypass.gamemode.<world>`, `mv.bypass.playerlimit.<world>` and
  `multiverse.core.spawn[.self|.other].<world>`, each with its
  `prefix.*` wildcard
- Multiverse's events (`MVWorldLoadedEvent`, `MVWorldPropertyChangedEvent`,
  ...) for other plugins to listen to

Multiverse-Inventories:

- `config.yml` with every Multiverse-Inventories 5 option and its comments,
  and `groups.yml`; an older `config.yml` is migrated, its groups moving to
  `groups.yml`; on the first run a `default` group of the vanilla worlds
  sharing everything is made
- groups of worlds by name, wildcard (`world_*`) or regex (`r=...`), with
  `shares`, `disabled-shares` and a respawn `spawn` world and priority;
  conflicting groups are reported on start
- the player's data is saved and switched on world change, on game mode
  change with `enable-gamemode-share-handling`, on join with
  `apply-playerdata-on-join`, on quit and on respawn in another world, with
  `default-ungrouped-worlds`, `use-optionals-for-ungrouped-worlds`,
  `always-write-world-profile` and the bypass permissions
  `mvinv.bypass.world.<world>`, `mvinv.bypass.group.<group>` and
  `mvinv.bypass.gamemode.<mode>` (with `enable-bypass-permissions`)
- the shares `inventory_contents`, `armor_contents`, `off_hand`,
  `ender_chest`, `hit_points`, `max_hit_points`, `food_level`, `saturation`,
  `exhaustion`, `xp`, `lvl`, `total_xp`, `remaining_air`, `maximum_air`,
  `fall_distance`, `fire_ticks`, `potion_effects`, `bed_spawn` (validated
  with `validate-bed-anchor-respawn-location`) and the optional
  `last_location` (with `apply-last-location-for-all-teleports` and
  `reset-last-location-on-death`), their groups (`inventory`, `stats`,
  `hunger`, `all`, ...) and other names
- the player files in Multiverse-Inventories' layout and JSON format:
  `worlds/<world>/<player>.json` and `groups/<group>/<player>.json` with
  one section per game mode, `players/<uuid>.json` and `playernames.json`;
  files from a Bukkit or Paper server are read, and a player who changed
  their name keeps their files
- items and inventory holders cannot go through portals between worlds
  that do not share inventories; creative inventory changes are refused
  for a few ticks after a world change, against item duping
- Multiverse-Inventories' share handling events
  (`WorldChangeShareHandlingEvent`, `GameModeChangeShareHandlingEvent`, ...)
  for other plugins to listen to and cancel

Multiverse-Portals:

- `config.yml` with every Multiverse-Portals 5.3 option and its comments, and
  `portals.yml` with every portal property; Multiverse-Portals 4 files are
  migrated
- `/mvp create`, `/mvp list`, `/mvp info`, `/mvp modify`, `/mvp select`,
  `/mvp remove`, `/mvp wand`, `/mvp debug` and `/mvp config`, with tab
  completion and the Multiverse 4 aliases (`/mvpc`, `/mvpl`, ...)
- selecting a portal's box with the wand (`wand-material`, a wooden pickaxe
  by default): left click the first corner, right click the second
- the `multiverse-destination`, `command` (`op:`, `console:`, `%player%`,
  `%world%`) and `server` (BungeeCord) action types
- the `p:<portal>[:<direction>]` and `rp:<portal>,...` destinations
- the permission nodes `multiverse.portal.*`, and per portal
  `multiverse.portal.access.<portal>`, `multiverse.portal.fill.<portal>` and
  `multiverse.portal.exempt.<portal>`

## Not yet

- Every `/mv` command not listed above; they are being ported.
- **Using Multiverse portals** (walking into one, nether portal fills, the
  bucket and flint-and-steel fills, entry fees, cooldowns, frames): being
  ported.
- **WorldEdit selections for `/mvp create`**: golem's WorldEdit
  (golem-mc/worldedit) offers a player's selection only through its
  `core.API` service, and importing that package would build WorldEdit and
  its SQLite driver into every server with Multiverse. Portals uses its own
  wand until WorldEdit offers a lightweight selection interface.
- **Bonus chests** (`/mv create --generate-bonus-chest`): golem cannot
  create a world with one yet
  ([golem-mc/golem#101](https://github.com/golem-mc/golem/issues/101)); the
  command says so.
- **Game rules per world** (`/mv clone` and `/mv regen` keep or reset
  them, `/mv gamerule set` sets them for one world): golem worlds share one
  set of game rules, so a rule set or reset in one world changes it in all
  ([golem-mc/golem#65](https://github.com/golem-mc/golem/issues/65)).
- **Spawn tick rates and limits** (`spawning.<category>.tick-rate` and
  `spawn-limit`, set with `/mv entity-spawn-config modify`): kept in
  `worlds.yml` but not applied, as golem has no per-world spawn rates or
  limits and no entity spawn categories yet
  ([golem-mc/golem#70](https://github.com/golem-mc/golem/issues/70)); until
  then the categories `/mv purge-entities` uses are a table of vanilla
  26.3's.

- **Multiverse-Inventories' commands** (`/mvinv`): being ported.
- **The `advancements` and `recipes` shares**: their saved data is kept in
  the player files but not switched, as golem cannot list the advancements
  or award criteria silently
  ([golem-mc/golem#97](https://github.com/golem-mc/golem/issues/97)) and has
  no recipe book API for plugins
  ([golem-mc/golem#98](https://github.com/golem-mc/golem/issues/98)).
- **The `game_statistics` share**: kept in the player files but not yet
  switched; it is being ported.
- **The `economy` share**: needs golem-mc/vault, the economy service; until
  then it warns once that there is no economy.
- **Importing from MultiInv, WorldInventories and PerWorldInventory**
  (Multiverse-InventoriesImporter), the PlaceholderAPI placeholders and the
  LuckPerms world group context: golem has no such plugins.

## Installing

```sh
golem-launcher plugin add github.com/golem-mc/multiverse
```

Or build a server with Multiverse built in: `mise run dev`, or
`go build ./cmd/server`.

## Configuration

Multiverse keeps its files in `plugins/multiverse-core/`. To move from a
Bukkit or Paper server, copy `config.yml` and `worlds.yml` from
`plugins/Multiverse-Core/` there. Multiverse-Inventories keeps its own in
`plugins/multiverse-inventories/`: copy `plugins/Multiverse-Inventories/`'s
`config.yml`, `groups.yml` and the `worlds`, `groups` and `players` folders
there. Multiverse-Portals keeps `config.yml` and
`portals.yml` in `plugins/multiverse-portals/` (from
`plugins/Multiverse-Portals/`).

## License

BSD-3-Clause, like Multiverse. See `LICENSE`.
