# juju-watch Architecture

## Data Flow

```text
poll Juju CLI
  -> parse JSON
  -> normalize Juju state
  -> build graph model
  -> compare with previous graph
  -> calculate diff
  -> update target layout
  -> animate current positions toward target positions
  -> render TUI frame
```

## Packages

- `cmd/juju-watch`: process entrypoint.
- `internal/cli`: flag parsing and user configuration.
- `internal/juju`: Juju command construction, execution, parsing, and polling.
- `internal/domain`: normalized model state, stable IDs, graph objects, and events.
- `internal/diff`: graph diffing by stable ID.
- `internal/layout`: deterministic schema layout and optional layout backends.
- `internal/animation`: Harmonica-backed movement state.
- `internal/tui`: Bubble Tea model, update loop, canvas renderer, keys, styles, and inspector.

## Stable Identity

Every object has a stable ID. Rendering, selection, diffing, layout preservation, and animation are all keyed by these IDs.

```text
model:<model-name>
app:<model-name>:<app-name>
unit:<model-name>:<unit-name>
machine:<model-name>:<machine-id>
relation:<model-name>:<endpoint-a>:<endpoint-b>
storage:<model-name>:<storage-id>
```

The TUI never treats a fresh poll as a brand-new scene. It computes a graph diff, preserves surviving object identity, updates changed objects, and only recalculates layout when topology changes.

## Polling

`juju-watch` polls `juju status -m <model> --format=json --relations --storage` on a configurable interval. Polling is executed outside the Bubble Tea update loop through commands, and results enter the UI as messages.

Poll errors do not clear the current graph. The UI keeps showing the last valid state and exposes the last error in the status bar and inspector/events.

## Layout

The default layout is deterministic schema layout:

- Applications are grouped by relation depth.
- Units are kept attached to their application.
- Machines are emphasized in the machines view.
- Relations are drawn as simple orthogonal edges.
- Existing targets are preserved across status-only updates.

Graphviz mode is an optional backend that shells out to `dot -Tplain`. Force layout is reserved for future experiments and must be seeded from existing coordinates to avoid visual churn.

## Rendering

Rendering uses a terminal cell canvas for graph primitives and Lip Gloss for styling panels, borders, status colors, and layout. Health color has priority over object type. Selection and recent changes are separate visual states so operational severity remains readable.
