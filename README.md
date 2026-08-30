# doppelcat

`doppelcat` watches one local UTF-8 text file, shows each stable update as a
single-column line diff, and supports lightweight terminal editing.

Requires Go 1.24 or newer.

```console
go build -o doppelcat ./cmd/doppelcat
doppelcat <file>
```

Run `doppelcat --help` for the fixed key bindings. Builds can inject a version
with `-ldflags "-X main.version=v1.0.0"`. The program has no configuration,
network access, telemetry, or Git integration.
