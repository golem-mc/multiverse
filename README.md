# Multiverse for golem

An unofficial port of [Multiverse](https://github.com/Multiverse) (Core, Inventories, Portals and NetherPortals) to [golem](https://github.com/golem-mc/golem). It is not made or endorsed by the Multiverse Team.

Commands, flags, options, permissions and config files follow Multiverse 5; see the [Multiverse documentation](https://mvplugins.org/) for full usage.

## Installing

```sh
golem-launcher plugin add github.com/golem-mc/multiverse
```

Optional modules (each requires the core):

```sh
golem-launcher plugin add github.com/golem-mc/multiverse/inventories
golem-launcher plugin add github.com/golem-mc/multiverse/portals
golem-launcher plugin add github.com/golem-mc/multiverse/netherportals
```

## What's included

- **World management** ([Core](https://mvplugins.org/core/)): create, import, load, unload, remove, delete, clone and regenerate worlds (`/mv create`, `/mv import`, `/mv load`, `/mv unload`, `/mv remove`, `/mv delete`, `/mv clone`, `/mv regen`), including custom generators.
- **World inspection and admin**: list and inspect worlds, edit config, reload, confirm destructive actions and generate debug dumps (`/mv list`, `/mv info`, `/mv who`, `/mv whoall`, `/mv coordinates`, `/mv config`, `/mv reload`, `/mv confirm`, `/mv debug`, `/mv dumps`, `/mv version`, `/mv help`).
- **Teleportation and anchors**: teleport players across worlds, manage world spawns and anchors, and use all Multiverse destination types (`/mv teleport`, `/mv spawn`, `/mv setspawn`, `/mv check`, `/mv anchor`).
- **World properties and access**: configure and enforce game mode, flight, respawn, hunger, auto-heal, PvP, difficulty, weather, advancements, spawning, portal rules, player limits, blacklists, entry fees (items or [Vault](https://github.com/golem-mc/vault)) and chat prefixes (`/mv modify`, `/mv meta`, `/mv entity-spawn-config`).
- **World utilities**: manage game rules, world borders, entity purging and generator listings (`/mv gamerule`, `/mv worldborder`, `/mv purge-entities`, `/mv purge-all-entities`, `/mv generators`).
- **Per-world inventories and stats** ([Inventories](https://mvplugins.org/inventories/)): separate inventories, ender chests, health, hunger, XP, potion effects, statistics, bed spawns, last locations and Vault balances by world, group and game mode.
- **Inventory groups and inspection**: create and edit world groups, view or edit live and stored player inventories, give items and run bulk edits (`/mvinv list`, `/mvinv info`, `/mvinv group`, `/mvinv create-group`, `/mvinv delete-group`, `/mvinv view`, `/mvinv modify`, `/mvinv give`, `/mvinv bulkedit`, `/mvinv reload`, `/mvinv config`).
- **Custom portals** ([Portals](https://mvplugins.org/portals/)): select regions with the portal wand and build portals that teleport to destinations, run commands or transfer BungeeCord servers (`/mvp create`, `/mvp list`, `/mvp info`, `/mvp modify`, `/mvp select`, `/mvp remove`, `/mvp wand`, `/mvp debug`, `/mvp config`).
- **Portal access and fills**: per-portal permissions, cooldowns, entry fees, custom portal frames filled with nether portal blocks, water or lava, and portal destinations.
- **Nether and End portal linking** ([NetherPortals](https://mvplugins.org/netherportals/)): link each world's nether and end portals to any other world, with auto-linking, coordinate scaling and bounceback (`/mvnp link`, `/mvnp unlink`, `/mvnp list`).
- **Permissions and Multiverse 4 migration**: Multiverse 5 permission nodes, legacy command aliases and automatic migration of Multiverse 4 configs and player data.

## Not yet

- Vehicles and mobs walking into a portal ([golem-mc/golem#120](https://github.com/golem-mc/golem/issues/120)).
- Keeping non-vanilla nether portal frames from collapsing on adjacent block updates ([golem-mc/golem#143](https://github.com/golem-mc/golem/issues/143)).
- WorldEdit selections for `/mvp create` (uses the built-in portal wand).
- Bonus chests on world creation ([golem-mc/golem#101](https://github.com/golem-mc/golem/issues/101)).
- Per-world game rules ([golem-mc/golem#65](https://github.com/golem-mc/golem/issues/65)).
- Natural block drops when clearing the End arrival platform ([golem-mc/golem#118](https://github.com/golem-mc/golem/issues/118)).
- Switching `advancements` and `recipes` shares ([golem-mc/golem#97](https://github.com/golem-mc/golem/issues/97), [golem-mc/golem#98](https://github.com/golem-mc/golem/issues/98)).
- Third-party integrations: Multiverse-InventoriesImporter, PlaceholderAPI placeholders and LuckPerms world-group contexts.

## Configuration

Config and data files live under `plugins/multiverse-core/`, `plugins/multiverse-inventories/`, `plugins/multiverse-portals/` and `plugins/multiverse-netherportals/`. To migrate from Bukkit or Paper, copy the corresponding `plugins/Multiverse-*/` files (`config.yml`, `worlds.yml`, `groups.yml`, `portals.yml`, `links.yml` and player data folders) into those directories.

## License

BSD-3-Clause, like Multiverse. See [LICENSE](LICENSE).
