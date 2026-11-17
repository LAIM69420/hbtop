# hbtop

A Braille dot-matrix scoreboard for CPU, GPU, memory, disk, network and LLM
inference metrics. Designed for a shallow herdr pane with a 90s-DMD /
modern-colour look.

## Build

```bash
make build
```

Go is installed at `~/.local/go/bin/go` and linked into `~/.local/bin/go`.

## Run

```bash
./hbtop
# or a specific config:
./hbtop configs/default.toml
```

## Keys

- `Tab` — cycle center modules
- `1` / `2` — PP / TG leaderboard
- `l` — toggle PP/TG leaderboard
- `i` — LLM view
- `p` — process view
- `m` — combined / model-only / run-only leaderboard
- `q` / `Esc` — quit

## Display

- Full-screen Braille dot matrix, animated neon palette.
- **Dashboard:** CPU/GPU left, center module in the middle, MEM/DSK right.
- **Center modules:**
  - TG leaderboard (default)
  - PP leaderboard
  - LLM inference: model name, last-run PP/TG, live stats during inference
  - Process list: PID, name, user, CPU%, MEM% in regular text
- **Action takeovers:** loading, ready, jackpot, new high score — full grid.
- **Attract mode:** optional (`attract_enabled`), disabled by default.

## Wordmarks & no-model attract

Provider wordmarks are rendered from config:

```toml
[[provider]]
match = "lfm"
wordmark = "LFM"
palette = ["#00c2ff", "#0077ff"]
# anim = "assets/lfm.branim.json"   # optional branim animation

[[provider]]
match = "qwen"
wordmark = "QWEN"
palette = ["#615ced", "#a26bff"]

[[provider]]
match = "ling"
wordmark = "LING"
palette = ["#00d4ff", "#0080ff"]
```

- While **no model is loaded**, the full grid runs a logo attract loop cycling
  all providers with materialize / scanline / bounce effects.
- **Model load and model switch** play the provider wordmark full-grid, then
  `READY`.
- The **LLM idle view** shows a small animated wordmark above the model name.
- Optional `.branim.json` files in `assets/` override the Go-native wordmark.

## Leaderboards

- Top 10 PP and TG scores.
- Combined, model-only, or run-only views (`m`).
- Persisted at `~/.local/state/hbtop/leaderboard.json`.
- New #1 triggers a full-grid takeover.

## LLM metrics

Polls llama.cpp:

- `/health`
- `/props` (`model_alias`)
- `/slots` (`is_processing`)
- `/metrics` (`prompt_tokens_seconds`, `predicted_tokens_seconds`,
  counters for totals)

Default endpoint: `http://127.0.0.1:8081`.

## GPU

Uses `rocm-smi --json` for AMD GPUs: usage, VRAM, temperature.

## Configuration

The app looks for `$HBTOP_CONFIG`, then `~/.config/hbtop/theme.toml`, then
`configs/default.toml`.

```toml
[display]
default_mode = "dashboard"
attract_enabled = false
attract_idle_ms = 10000
attract_step_ms = 4000
action_ms = 3000

[palette]
colors = ["#ffb000", "#ff2d95", "#00e5ff", "#7dff00", "#ff5e00", "#9b5cff"]
cycle_ms = 6000
brightness = 1.0

[inference]
enabled = true
url = "http://127.0.0.1:8081"
poll_ms = 250
timeout_ms = 500

[gpu]
enabled = true
poll_ms = 1000
```
