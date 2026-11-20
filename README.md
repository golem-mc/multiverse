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
  `player-limit`, `world-blacklist` and item entry fees (charged and paid
  out), with `teleport-intercept`; `join-destination` and
  `first-spawn-override` on join
- the permission nodes `multiverse.core.*`, and per world
  `multiverse.access.<world>`, `multiverse.exempt.<world>`,
  `mv.bypass.gamemode.<world>`, `mv.bypass.playerlimit.<world>` and
  `multiverse.core.spawn[.self|.other].<world>`, each with its
  `prefix.*` wildcard
- Multiverse's events (`MVWorldLoadedEvent`, `MVWorldPropertyChangedEvent`,
  ...) for other plugins to listen to

## Not yet

- Every `/mv` command not listed above; they are being ported.
- **Entry fees paid with an economy** (`entry-fee.currency: '@vault-economy'`):
  needs golem-mc/vault, the Vault plugin that will provide the shared
  economy service.
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

## Installing

```sh
golem-launcher plugin add github.com/golem-mc/multiverse
```

Or build a server with Multiverse built in: `mise run dev`, or
`go build ./cmd/server`.

## Configuration

Multiverse keeps its files in `plugins/multiverse-core/`. To move from a
Bukkit or Paper server, copy `config.yml` and `worlds.yml` from
`plugins/Multiverse-Core/` there.

## License

BSD-3-Clause, like Multiverse. See `LICENSE`.
