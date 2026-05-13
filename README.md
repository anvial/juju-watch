# juju-watch

Live terminal topology viewer for Juju models.

`juju-watch` turns `juju status` output into a stable, searchable graph so you can watch applications, units, machines, storage, relations, and operational status from one terminal UI. It polls the Juju CLI, keeps surviving nodes anchored between refreshes, highlights changes, and gives you focused views for topology, machines, problems, and events.

## Features

- Live polling of a Juju model with manual refresh and pause/resume controls.
- Stable topology layout for applications, units, machines, storage, and relations.
- Relation arrows rendered from provider to consumer.
- Status-aware rendering for active, waiting, blocked, error, and maintenance states.
- Search and focus for applications, units, machines, relations, and status text.
- Dedicated views for topology, machines, problems, and event history.
- Optional debug logging for troubleshooting TUI behavior.

## Prerequisites

- Go 1.24.2 or newer.
- The Juju CLI installed and available on your `PATH`.
- Access to a bootstrapped Juju controller and model.
- Graphviz `dot` only if you use `--layout graphviz`.

## Installation

Install the latest version with Go:

```bash
go install github.com/anvial/juju-watch/cmd/juju-watch@latest
```

Make sure your Go binary directory is on `PATH`. For most setups:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Run from source without installing:

```bash
git clone https://github.com/anvial/juju-watch.git
cd juju-watch
go run ./cmd/juju-watch -m <model>
```

## Quick Start

Start watching a model:

```bash
juju-watch -m prod
```

Use a custom polling interval:

```bash
juju-watch -m prod --interval 10s
```

Open a specific view:

```bash
juju-watch -m prod --view topology
juju-watch -m prod --view machines
juju-watch -m prod --view problems
juju-watch -m prod --view events
```

Focus an application or unit on startup:

```bash
juju-watch -m prod --focus postgresql
juju-watch -m prod --focus postgresql/0
```

Try a different layout backend:

```bash
juju-watch -m prod --layout schema
juju-watch -m prod --layout graphviz
```

Disable animations or enable debug logging:

```bash
juju-watch -m prod --no-animation
juju-watch -m prod --debug --log-file juju-watch.log
```

Relations and storage are enabled by default. Internally, the app polls:

```bash
juju status -m <model> --format=json --relations --storage
```

`juju-watch` does not use `juju status --watch`; polling is controlled by the TUI so refreshes, pauses, errors, diffs, and animation state can be handled consistently.

## Terminal Preview

```text
+--------------------+              +--------------------+
| postgresql       * |              | api-server       ! |
| active             |---- db ----> | blocked            |
| units: 3           |              | units: 2           |
| * postgresql/0     |              | * api-server/0     |
| x postgresql/1     |              | ~ api-server/1     |
+--------------------+              +--------------------+

Inspector
application: api-server
status: blocked
message: missing relation
```

## Keyboard Shortcuts

| Key | Action |
| --- | --- |
| `q` | Quit |
| `r` | Refresh now |
| `space` | Pause or resume polling |
| `tab` | Switch view |
| `/` | Search |
| `f` | Focus selected node |
| `arrow keys` | Move selection |
| `h`, `j`, `k`, `l` | Pan canvas |
| `?` | Toggle help |
| `esc` | Close search or help |

## CLI Options

| Option | Description |
| --- | --- |
| `-m <model>` | Juju model to watch |
| `--model <model>` | Juju model to watch |
| `--interval <duration>` | Poll interval, default `5s` |
| `--view <name>` | Initial view: `topology`, `machines`, `problems`, or `events` |
| `--layout <name>` | Layout mode: `schema`, `graphviz`, or `force` |
| `--focus <query>` | Initial application or unit focus |
| `--relations` | Include relations in `juju status`, enabled by default |
| `--storage` | Include storage in `juju status`, enabled by default |
| `--all-models` | Reserved for future multi-model overview; not implemented yet |
| `--no-animation` | Disable animations |
| `--debug` | Enable debug logging |
| `--log-file <path>` | Debug log file path |

## Current Limitations

- Multi-model overview with `--all-models` is not implemented yet.
- `schema` is the default layout and the only fully implemented layout mode.
- `graphviz` is an extension point and requires `dot`.
- `force` is reserved for future layout experiments.
- Rendering is terminal-cell based and intentionally conservative for readability.

## Roadmap

- Multi-model overview.
- Cross-model relation visualization.
- Network spaces and storage-focused views.
- Machine health view.
- Relation impact analysis.
- Focus, search, and filtering improvements.
- Snapshot export as text, JSON, SVG, or Excalidraw-like scene.
- Mouse support.
- Plugin-style command collectors.

## Development

Run the test suite:

```bash
go test ./...
```

Run the app locally:

```bash
go run ./cmd/juju-watch -m <model>
```
