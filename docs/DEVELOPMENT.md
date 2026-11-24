# Development

## Requirements

- Linux (collectors read `/proc` and `/sys/class/hwmon`).
- Go 1.27+.
- Optional: `rocm-smi` for AMD GPU stats.
- Optional: `herdr` or any terminal for manual testing.

## Build and run

```bash
make build          # go build -o hbtop ./cmd/hbtop
make run            # build then run
make install        # install to ~/.local/bin
make clean
```

Run with a specific config:

```bash
./hbtop configs/default.toml
```

## Checks

```bash
go build ./...
go vet ./...
```

There is no test suite yet.

## Project layout

See [ARCHITECTURE](ARCHITECTURE.md) for the package map and data flow.

## Adding a provider wordmark

Add a `[[provider]]` block to your config:

```toml
[[provider]]
match = "mixtral"
wordmark = "MIX"
palette = ["#ff7a00", "#ffcc00"]
```

`match` is a case-insensitive substring of the model path. If no provider
matches, hbtop derives a short wordmark from the model filename.

## Adding a branim animation

1. Design or generate a `.branim.json` animation (see
   [BRANIM](BRANIM.md)).
2. Put it in `assets/`.
3. Point a provider at it:

```toml
[[provider]]
match = "qwen"
wordmark = "QWEN"
palette = ["#615ced", "#a26bff"]
anim = "assets/qwen.branim.json"
```

The branim file's own dimensions and fps are respected; hbtop centres it in the
target area and applies the provider palette.

## Tuning the centre modules

- `internal/ui/ui.go` holds the dashboard, stat drawing, leaderboard, LLM view,
  process view, actions, and wordmarks.
- `internal/ui` is intentionally one file for now; split it if it grows.
- The center view enum is `centerView`; key handling lives in
  `cmd/hbtop/main.go`.

## Debugging a live instance

With herdr:

```bash
herdr pane read <pane-id> --source visible --lines 40 --format text
herdr pane read <pane-id> --source visible --lines 40 --format ansi
```

For the inference watcher, the fastest check is the server itself:

```bash
curl -s localhost:8081/health
curl -s localhost:8081/props | jq .model_alias
curl -s localhost:8081/slots | jq '.[0].is_processing'
curl -s localhost:8081/metrics | grep llamacpp
```

## Style

- Standard Go formatting (`gofmt`).
- Keep collectors side-effect free and cheap; they run on tickers.
- The UI owns all mutable display state; do not call tcell from collectors.
