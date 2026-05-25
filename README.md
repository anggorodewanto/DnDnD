# DnDnD

A Discord-native D&D 5e play assistant: a Go bot + service that runs encounters,
combat, exploration, and map rendering for a campaign, with a web dashboard for
DMs.

## Documentation

- **[Building Battle Maps with Tiled](docs/tiled-maps.md)** — author full battle
  maps in the [Tiled](https://www.mapeditor.org/) editor with real game tilesets
  and import them so encounters render with actual tile art in the web preview
  and Discord `#combat-map` posts.
- [Running locally](docs/local-run.md) — environment setup and how to start the
  service.
- [Playtest quickstart](docs/playtest-quickstart.md) — fresh checkout to a live
  `/move`-ready encounter.
- [Playtest checklist](docs/playtest-checklist.md) — scenarios to walk each session.
- [Testing](docs/testing.md) — the test pyramid, fixtures, and coverage rules.
- [Spec](docs/dnd-async-discord-spec.md) · [Phases](docs/phases.md)

## Maps

Maps can be authored two ways:

- **In the web map editor** — paint terrain, walls, lighting, elevation, and
  spawn zones directly.
- **Imported from Tiled** — build a full-art map with real tilesets and import
  the `.tmj` plus its image files in one step. See the
  [Tiled maps guide](docs/tiled-maps.md).

## Development

See [CLAUDE.md](CLAUDE.md) for the development process (orchestrated subagents,
red/green TDD, coverage gates). Common commands:

```sh
make run          # run the service
make cover-check  # run tests with coverage gates
```
