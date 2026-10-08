# OTS Creator

Object editor for Open Tibia clients (Tibia.dat / Tibia.spr, OBD import/export), built with
[Wails v3](https://v3.wails.io) (Go) and Svelte 5.

## Development

Requirements: Go 1.25+, Node 22+, `wails3` CLI (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`).

```sh
wails3 task dev        # run with hot reload
wails3 task build      # production build into bin/
wails3 task test       # Go + frontend unit tests
```

## Releases

Push a version tag to build and publish a release; installed apps update from it.

```sh
git tag v1.2.3 && git push origin v1.2.3
```

## License

Apache License 2.0 with the Commons Clause (see `LICENSE` and `NOTICE`): you may use,
modify and share OTS Creator, but you may not sell it. Derived works must keep the
copyright notices and credits.

Copyright (c) 2026 Marcin Jałocha (nekiro)
