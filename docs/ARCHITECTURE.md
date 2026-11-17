# Architecture

hbtop is a single Go binary with a small set of internal packages. There is no
daemon or database; everything runs in one process and refreshes on tickers.

## Package layout

```
cmd/hbtop/            main loop, tickers, channel wiring
internal/braille/     2x4 dot canvas + 5x7 pixel font
internal/collector/   CPU/mem/disk/net, GPU, processes, temperatures
internal/inference/   llama.cpp watcher + event detection
internal/leaderboard/ persistent PP/TG high-score store
internal/theme/       TOML configuration
internal/anim/        .branim.json loader
internal/ui/          renderer, dashboard, chips, center modules
```

## Data flow

```
collector (1s) ─┐
GPU (1s) ───────┤
temperatures ───┼──> ui.UI ──> Braille canvas ──> tcell ──> terminal
processes (2s) ─┤
inference (250ms) ─┘         │
                             └─> leaderboard.Store (on inference finish)
```

Each collector runs in its own goroutine and sends snapshots through a buffered
channel. The main loop owns all UI state and rendering; collectors never touch
the screen.

## Rendering

- The terminal is treated as a grid of Braille cells. Each cell holds a 2×4 dot
  block (U+2800–U+28FF).
- `internal/braille.Canvas` stores one byte per cell and supports rectangular
  clipping.
- Text is drawn with a 5×7 bitmap font (`internal/braille/font.go`).
- The UI draws into a full-screen canvas, then blits each cell to tcell.
- Colour is decided per cell at blit time:
  - global palette gradient by default;
  - provider palette while a wordmark is active;
  - chip zone override for CPU/RAM (temperature gradient).

## Display states

```
dashboard ──idle/attract──> attract
    │
    └──event──> action (READY / JACKPOT / NEW #1 / LOADING)
                   │
                   └──timeout──> previous center view
```

- **Dashboard** is the default. Left = CPU chip, right = RAM module, centre =
  active center module.
- **Attract** is optional for stats. When `/health` is down, the offline
  dashboard keeps the CPU/RAM chips visible and cycles provider wordmarks in
  the centre.
- **Action** takeovers draw over the whole grid and restore the previous view
  afterwards.

## Center modules

`internal/ui` keeps a `centerView` enum:

- `viewLeaderboardTG` (default)
- `viewLeaderboardPP`
- `viewLLM`
- `viewProcesses`

Keys change the enum; inference start switches to `viewLLM` and restores the
previous view on finish. Process view is drawn as ASCII after the Braille blit.

## CPU / RAM chips

`drawCPU` and `drawRAM` choose a layout based on available dot height:

- **Tall panes:** full chip package with pins and an inner die; usage and
  temperature are two large lines.
- **Short panes:** edge-to-edge outline and one compact line (`95% 59°`) at
  scale 2 so the numbers stay legible.

Chip colour is computed from temperature:

```
temp < 30        cyan
30 .. 50         cyan  -> yellow
50 .. 70         yellow -> orange
70 .. 90         orange -> red
> 90             red
```

If a sensor is missing, usage is used as a fallback input.

## Inference watcher

`internal/inference.Watcher` polls a llama.cpp server:

| Endpoint | Purpose |
| --- | --- |
| `/health` | online/offline |
| `/props` | `model_alias` |
| `/slots` | `is_processing` |
| `/metrics` | tokens/s gauges and cumulative counters |

- A failed `/health` is transient until three consecutive failures, so a busy
  server does not look offline.
- Prompt and generation throughput are computed from counter deltas between
  the last idle poll and the finish poll, which is robust even if intermediate
  polls are missed.
- Events: model ready, inference started, inference finished, offline.

## Leaderboards

`internal/leaderboard.Store` keeps four top-10 lists:

- `pp_model`, `pp_run`, `tg_model`, `tg_run`

Combined views merge the model and run lists, deduplicate identical
model+score pairs, and tag each row `M` or `R`. The store is saved to
`~/.local/state/hbtop/leaderboard.json` after each run.

## Wordmarks and branim

- Providers are matched by substring against the model path.
- Built-in wordmarks render text with the 5×7 font and a materialize /
  scanline / bounce effect.
- `.branim.json` files are mono braille frame sets; hbtop loads them and draws
  the current frame into the canvas at the file's fps. Palette colour is still
  applied by hbtop.

## Temperature sources

Linux hwmon:

- `k10temp` / `coretemp` / `cpu` → CPU
- `spd5118` / `dimm` / `ram` → RAM
- `amdgpu` → GPU
- `acpitz` → CPU fallback

GPU usage/VRAM still comes from `rocm-smi --json`.
