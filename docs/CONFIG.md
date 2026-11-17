# Configuration reference

hbtop reads a single TOML file. Search order:

1. `$HBTOP_CONFIG`
2. `~/.config/hbtop/theme.toml`
3. `configs/default.toml` (path relative to the working directory)

All sections are optional; missing values fall back to the defaults shown
below.

## `[theme]` — reserved

Colour names for a future theme engine. Currently unused by the renderer; the
palette in `[palette]` and provider palettes are what actually paint the UI.

```toml
[theme]
background = "default"
foreground = "#e0e0e0"
accent = "#ffb000"
warning = "#ff5555"
```

## `[layout]` — reserved

```toml
[layout]
rows = 4
columns = 80
```

Currently unused; the layout adapts to the terminal size.

## `[display]`

```toml
[display]
default_mode = "dashboard"   # dashboard | attract
attract_enabled = false
attract_idle_ms = 10000
attract_step_ms = 4000
action_ms = 3000
```

| Key | Default | Meaning |
| --- | --- | --- |
| `default_mode` | `dashboard` | Starting mode. `attract` starts in the attract cycle. |
| `attract_enabled` | `false` | Enable the stat attract loop after idle. The no-model provider attract always runs when the server is offline. |
| `attract_idle_ms` | `10000` | Idle time before the stat attract loop starts. |
| `attract_step_ms` | `4000` | Time per attract item / provider wordmark. |
| `action_ms` | `3000` | Duration of READY / JACKPOT / new-high-score takeovers. |

## `[palette]`

Global neon palette. Each cell picks a colour from the palette based on screen
position and time, producing the moving gradient.

```toml
[palette]
colors = ["#ffb000", "#ff2d95", "#00e5ff", "#7dff00", "#ff5e00", "#9b5cff"]
cycle_ms = 6000
brightness = 1.0
```

| Key | Default | Meaning |
| --- | --- | --- |
| `colors` | neon set above | Hex colours. Any number of entries. |
| `cycle_ms` | `6000` | Reserved; animation speed is currently frame-based. |
| `brightness` | `1.0` | Multiplier applied to palette colours. |

## `[inference]`

llama.cpp endpoint and polling.

```toml
[inference]
enabled = true
url = "http://127.0.0.1:8081"
poll_ms = 250
timeout_ms = 1500
```

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `true` | Reserved; the watcher currently always polls. |
| `url` | `http://127.0.0.1:8081` | Base URL. A trailing `/metrics` is stripped if present. |
| `poll_ms` | `250` | Poll interval for `/health`, `/props`, `/slots`, `/metrics`. |
| `timeout_ms` | `1500` | Per-request timeout. Increase for heavily loaded servers. |

A single failed `/health` is treated as transient; hbtop declares the server
offline only after three consecutive failures.

## `[gpu]`

```toml
[gpu]
enabled = true
poll_ms = 1000
```

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `true` | Reserved; the GPU collector currently always polls. |
| `poll_ms` | `1000` | `rocm-smi` poll interval. |

## `[[provider]]`

One or more provider definitions. The first whose `match` is a case-insensitive
substring of the model path wins.

```toml
[[provider]]
match = "lfm"
wordmark = "LFM"
palette = ["#00c2ff", "#0077ff"]
# anim = "assets/lfm.branim.json"
```

| Key | Meaning |
| --- | --- |
| `match` | Substring matched against the model path. |
| `wordmark` | Short wordmark shown in the no-model attract and LLM view. |
| `palette` | Colours used while this provider's wordmark is on screen. |
| `anim` | Optional `.branim.json` file; overrides the built-in wordmark animation. |

If no provider matches, hbtop derives a wordmark from the model basename
(for example `GrootTeam_Groot-19B-…` → `GROOT`) and uses the global palette.

## Minimal example

```toml
[display]
attract_enabled = false
action_ms = 3000

[palette]
colors = ["#ffb000", "#ff2d95", "#00e5ff", "#7dff00"]

[inference]
url = "http://127.0.0.1:8081"
poll_ms = 250
timeout_ms = 1500

[gpu]
poll_ms = 1000

[[provider]]
match = "lfm"
wordmark = "LFM"
palette = ["#00c2ff", "#0077ff"]

[[provider]]
match = "qwen"
wordmark = "QWEN"
palette = ["#615ced", "#a26bff"]

[[provider]]
match = "ling"
wordmark = "LING"
palette = ["#00d4ff", "#0080ff"]
```
