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
  form
- the vanilla worlds are imported as `world`, `world_nether` and
  `world_the_end` (after `level-name`), worlds made by other plugins too;
  worlds with `auto-load` load on start
- `/mv list`, `/mv info`, `/mv config`, `/mv debug`, `/mv reload`,
  `/mv confirm`, `/mv help`, `/mv version`, with their flags (`--page`,
  `--filter`, `--raw`), tab completion and the Multiverse 4 aliases
  (`/mvlist`, `/mvinfo`, ...) when `show-legacy-aliases` is on
- `/mv create`, `/mv import`, `/mv load`, `/mv unload`, `/mv remove`,
  `/mv delete`, `/mv clone` and `/mv regen`, with their flags (`--seed`,
  `--generator`, `--properties`, `--force-spawn-position`,
  `--remove-players [destination]`, `--no-save`, `--keep-files`, ...), the
  `/mv confirm` step for delete and regen, and tab completion of unknown
  world folders for import
- the world (`w:`) destination for `--remove-players`
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
- **Generators by plugin name** (`-g Plugin:id`): golem has no way for a
  plugin to offer generators by name yet
  ([golem-mc/golem#67](https://github.com/golem-mc/golem/issues/67)). Until
  then a generator plugin can call `core.RegisterGenerator` from its `init`.

- **World types and creation options** (`/mv create --world-type flat`,
  `large_biomes`, `amplified`, `--no-structures`, `--biome`,
  `--generator-settings`, `--generate-bonus-chest`): golem cannot make such a
  plugin world yet ([golem-mc/golem#66](https://github.com/golem-mc/golem/issues/66));
  the command says so instead of making a different world.
- **Game rules per world** (`/mv clone` and `/mv regen` keep or reset
  them): golem worlds share one set of game rules
  ([golem-mc/golem#65](https://github.com/golem-mc/golem/issues/65)).

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
