# Porting Multiverse to golem: layout and conventions

This module is Multiverse (Java, Bukkit/Paper) ported to golem plugins (Go).
It follows Multiverse-Core 5.8's source, not older documentation. `go.mod`
requires a golem commit; move to a newer one with
`mise exec -- go get github.com/golem-mc/golem@main` and `mise run tidy`.

## Layout

| Path | What | Java counterpart |
|---|---|---|
| `multiverse.go` | The `multiverse-core` plugin: enables core, adds the commands | `MultiverseCore.onEnable` |
| `core/` | config.yml, worlds.yml, the world manager, messages, permissions, the command framework, the API the modules use | `config/`, `world/`, `locale/`, `permissions/`, `command/`, `display/` |
| `core/resources/` | Multiverse-Core's `multiverse-core_<lang>.properties`, unchanged | `src/main/resources` |
| `commands/` | The `/mv` subcommands, one file per Java command class (or a few related ones) | `commands/*Command.java` |
| `inventories/` | The `multiverse-inventories` plugin: config.yml, groups.yml, the sharables, the profile files and the share handling | Multiverse-Inventories |
| `internal/mvtest/` | Test harness over golem's `plugintest`, and cheap test generators (`Void`, `Floor`) | — |
| `portals/` | The `multiverse-portals` plugin: portals.yml, config.yml, `/mvp`, the `p:` and `rp:` destinations | Multiverse-Portals |
| `cmd/server/` | A server binary with every plugin built in | — |

## How the Java maps to Go

- **Config nodes** (`ConfigNode`, `NodeGroup`) are `core.Node` in a
  `core.Nodes` set holding `core.Values`. One table per file builds the
  load, the save (config.yml with its comments, worlds.yml as Bukkit writes
  it) and what `/mv config` and `/mv modify` can read and set.
- **World properties** that change the loaded world (`onLoadAndChange`)
  are hooks: `Worlds.OnChange(path, fn)` runs `fn` when the property
  changes and when the world loads.
- **Commands** (ACF) are `core.Cmd`: the subcommand path, aliases, legacy
  roots, permission, syntax, flags and completion. `/mv` is one golem
  command with a greedy argument; core parses flags as Multiverse's
  `CommandFlagsParser` does and completes them. Issuer-aware world
  arguments are `Ctx.WorldArg`.
- **Messages** are `core.M(key, "{placeholder}", value, ...)`, sent with
  `Issuer.Info` / `Error` / `Send`, which colour them as ACF's message types
  (info blue, error red) and turn `&` codes into text.
- **Events** (`MVWorldLoadedEvent`, ...) are golem plugin events defined
  with `Events().Define`, fired with `Core.Fire`.
- **Results** (`Attempt`, `FailureReason`) are Go errors; a
  `core.MessageError` carries the localized failure message.

## Deliberate differences

- A world's key is its golem dimension key, and a name maps to a key as
  Multiverse 5.7+ does on Paper: `level-name` is `minecraft:overworld`,
  `<level>_nether` and `<level>_the_end` the vanilla nether and end, any
  other name `minecraft:<lowercase name>`. A world's files are in
  `<level>/dimensions/<namespace>/<path>`, golem's layout, not a folder of
  their own next to the server.
- `/mv info`'s World UID is a UUID derived from the key: golem worlds have
  no UUID.
- `/mv help` lists the subcommands as ACF does, in a format close to but not
  exactly ACF's.
- `/mv delete` and `/mv regen` refuse the vanilla overworld, nether and end,
  which golem cannot unload. A clone copies a world's own folder (for the
  overworld, `dimensions/minecraft/overworld`, not the level directory) but
  not golem's `data/plugin_world.dat`; the copy takes the source's seed and,
  unless `--reset-world-config`, its spawn from `worlds.yml`.
- `/mv gamerule` names a rule by its key (`keep_inventory` or
  `minecraft:keep_inventory`) and also takes it with case and underscores
  ignored (`keepInventory`); a boolean rule takes only `true` or `false`,
  where ACF reads anything else as `false`.
- `/mv worldborder set` and `add` say "shrinking" when the border shrinks;
  Multiverse swaps the two messages.
- `/mv purge-all-entities <world> <categories>` fails when no category is
  valid, where Multiverse would purge every entity.
- Player selectors in `/mv tp` and the `pl:`/`b:` destinations work without
  arguments only (`@a`, `@p`, `@r`, `@s`, `@e` for players): golem gives
  plugins no selector parser.
- Multiverse-Inventories writes an item stack with data components in
  Paper's byte form (base64 of the gzipped NBT) even with
  `use-byte-serialization-for-inventory-data` off, as Bukkit's item meta map
  cannot carry them; a plain stack is written in Bukkit's map form. Of a
  Bukkit map from another server it reads the type, amount and damage.
- `maximum_air` is Bukkit's own setting, which vanilla fixes at 300: it is
  saved as 300 and applies nothing.
- Multiverse-Inventories keeps a player's profiles in memory while they are
  online and writes the files at once, rather than through its caches and a
  file queue; the cache options are kept in `config.yml` but unused. A
  `bed_spawn` is marked a bed or an anchor by the block at the respawn point
  when it is saved, as golem has no event for a player's respawn point
  changing.
- `/mvinv cache stats` counts the uses of the profiles kept for online
  players, under Multiverse-Inventories' three cache names.
- `/mvinv bulkedit` runs at once and reports when it is done, rather than
  in the background.
- A negative entry fee (a reward) says "deposited" when it is paid out;
  Multiverse says "charged" for both directions.
- `rp:` with no portal named picks any portal; Multiverse fails to find
  one. `/mvp list <world>` lists that world's portals also when
  `enforce-portal-access` is off, where Multiverse lists every portal.
- A portal's location set with `/mvp modify` takes effect at once;
  Multiverse finds the portal at its new place only after a reload.
- A `command` portal action without `op:` or `console:` runs only for
  players: golem's other entities cannot run commands.
