# hbtop PLAN

## Goal
Fork / build **btop++** (aristocratos/btop) from source and customize it to:
1. Edit the appearance of existing readouts (colors, symbols, box layout).
2. Add **LLM inference stats**.
3. Redesign overall layout for optimal horizontal function with limited vertical space (~4 lines per screen).

Do **not** make any changes yet — this is planning only.

---

## 1. Research findings

### Source & build
- Repo: `github.com/aristocratos/btop`, C++ project, Apache‑2.0.
- Build options via:
  - **Makefile**: flags like `VERBOSE`, `STATIC`, `QUIET`, `GPU_SUPPORT`, `RSMI_STATIC`, `ARCH`, `ADDFLAGS`, `CXX`.
  - **CMake** (community maintained): flags `-DBTOP_GPU=ON/OFF`, `-DBTOP_LTO=ON/OFF`, `-DBTOP_STATIC=ON/OFF`, `-DCMAKE_INSTALL_PREFIX=...`.
- Install deps: GCC/Clang ≥14, CMake, Ninja, Lowdown, Git, coreutils/sed.

### Config file (`btop.conf`)
- TOML at `~/.config/btop/themes/btop.conf` or `$XDG_CONFIG_HOME/btop/btop.conf`.
- Contains ~100 keys covering:
  - Theme: `theme_background`, `truecolor`, `force_tty`, `graph_symbol`, etc.
  - Presets (`presets = "cpu:1:default,proc:0:default ..."`).
  - Keybindings (`vim_keys`, hotkey for collapse/expand all processes #1564).
  - Graph symbols per box (CPU, mem, net, proc, temp, etc.).
- Editable via the UI or by hand‑editing.

### Themes
- Dozens of theme files under `themes/*.theme` (e.g., `monokai.theme`, `hotpurpletrafficlight.theme`).
- Each defines variables like `main_bg`, `cpu_box`, `mem_box`, `div_line`, etc.
- Layout is controlled by presets and config keys, not directly by theme files.

### Presets
- Format: `"box_name:P:G,box_name:P:G ..."` where P = position (0/1 for alternate), G = graph symbol.
- Default preset shows all boxes with default settings. Max 9 presets.
- Example from README:
  ```
  cpu:1:default,proc:0:default cpu:0:default,mem:0:default,net:0:default cpu:0:block,net:0:tty
  ```

### Recent activity (v1.4.8)
- Fixes atomic issues, GPU support for AMD/Intel, Apple Silicon temp reporting.
- New themes added; many contributors.

---

## 2. Desired layout

**Current default:** one column per metric, stacked vertically with default box sizes → likely too tall for limited vertical space (user wants ~4 lines).

**Proposed redesign:**
- **One column per metric**, but each readout is a *narrower* box so they stack into only 4 rows.
- Suggested 4‑line layout:
  - Line 1: CPU (cores + usage)
  - Line 2: Memory (used / free / cached / available)
  - Line 3: Disks (usage, free, IO speed)
  - Line 4: Network (download / upload) **+** LLM inference stats (model name, tokens/sec, GPU memory, latency/token, throughput).
- Horizontal bars inside each box for CPU/memory graphs; simple meters for disk/network.
- Use compact graph symbols (`block` or `tty`) to save vertical space.

---

## 3. LLM inference stats template

Add a new readout (or extend the "CPU" box) with these fields:

| Field | Description |
|-------|-------------|
| **model_name** | Name of the LLM model being inferred (e.g., `LLaMA-2`, `GPT‑4o`). |
| **prompt_tokens_per_sec** | Tokens processed per second under current prompt. |
| **gpu_memory_mb** | GPU memory used by inference. |
| **latency_per_token_ms** | Time taken to generate one token (ms). |
| **throughput_tokens_per_sec** | Total tokens generated / sec. |
| **queue_depth** | Number of pending requests in the queue. |

*Template layout suggestion:* a separate row at the bottom (line 4) titled **"Inference"** with these 6 columns side‑by‑side, or as sub‑boxes under CPU.

---

## 4. Build from source – step‑by‑step plan

1. **Clone**
   ```bash
   git clone https://github.com/aristocratos/btop.git
   cd btop
   ```
2. **Install dependencies** (example Ubuntu/Debian)
   ```bash
   sudo apt install cmake git g++ ninja-build lowdown coreutils sed
   ```
   macOS: `brew install cmake git llvm ninja lowdown`.
3. **Configure CMake** with desired flags:
   ```bash
   cmake -B build -G Ninja \
        -DBTOP_GPU=ON          # or OFF to disable GPU graphs
        -DBTOP_LTO=OFF         # link‑time optimization off if not needed
        -DBTOP_STATIC=OFF      # static linking (only libgcc/libstdc++)
        -DCMAKE_INSTALL_PREFIX=/opt/hbtop   # custom install prefix
   ```
   Or with Makefile flags: `make GPU_SUPPORT=false`, etc.
4. **Build**
   ```bash
   cmake --build build      # or make
   ```
5. **Install** (root only for non‑user dirs)
   ```bash
   sudo cmake --install build
   ```
6. **Create new preset** in `btop.conf` (see section 5) and install it:
   - Edit `$HOME/.config/btop/themes/btop.conf`.
7. **Add inference box**:
   - Either add a config key like `inference_box = "cpu"` or create a new variable in the theme file.
   - Update CMake if needed (e.g., `-DBTOP_INFERENCE=ON`).
8. **Test** on your terminal, check that 4 lines fit and appearance matches desired colors/symbols.

---

## 5. Customizing existing readouts

- Use `btop --config /path/to/btop.conf` to override defaults.
- Edit theme variables directly in a `.theme` file (e.g., change `main_bg`, `cpu_box`, `mem_box`, `graph_symbol_cpu`).
- Presets control which boxes are shown and their positions; you can add a new preset that shows only the compact set of boxes above.
- Keybindings (`vim_keys`, hotkey for collapse/expand all processes) can be tuned via config.

---

## 6. Themes & appearance

- Keep existing themes as base, but modify variables to match new layout:
  - `theme[main_bg]`, `theme[cpu_box]`, `theme[mem_box]`, etc.
  - Graph symbols: e.g., set `theme[cpu_start]="#A6E22E"` for a lighter CPU bar.
- New inference readout can get its own variables (`inference_fg`, `graph_symbol_inference`, etc.) in the theme file.

---

## 7. Next questions / decisions needed

1. **Terminal width**: How many columns do you want per line? (e.g., ~40 chars, or two side‑by‑side bars for CPU & memory?)
2. **Exact row count**: You mentioned ~4 lines total. Which rows should be kept?
   - Suggested: Line 1 = CPU, Line 2 = Memory, Line 3 = Disks, Line 4 = Network + Inference.
3. **Inference fields**: Confirm the six fields above or adjust/remove any.
4. **Preset name & ID**: Should we call it `compact_horiz` (or similar) and assign it a number 10‑12? Need to know if you want to keep existing presets unchanged.
5. **Static linking / GPU**: Do you need static binaries or GPU support on?
6. **macOS setuid/setcap**: Should we enable `gmake setuid` and `make setcap` for Linux/macOS?

---

## 8. Risks & dependencies

- Requires CMake/Ninja/Lowdown/Clang/GCC ≥14.
- GPU monitoring needs NVIDIA drivers + `nvidia-ml` (or AMD ROCm SMI).
- macOS build needs Homebrew and lowdown; Linux/macOS/FreeBSD/NetBSD/OpenBSD have separate makefiles.
- Adding new boxes may break existing themes that reference box variables; update theme files accordingly.

---

*This plan has been updated with new sections on Braille Unicode dot matrix layout and herdr panes; no source code or build artifacts were changed.*

## 9. Braille Unicode Dot Matrix Layout

- **Concept:** Treat herdr's pain space as one large horizontal dot matrix grid. Use only Braille Unicode characters (U+2800–U+28FF, U+25A0–U+25FF, etc.) to create a dense grid of dots that can serve as cells for the scoreboard.
- **Why braille:** Braille patterns provide high contrast and ample spacing when rendered in terminals; they avoid emoji (which cost 2‑3 tokens) and give enough area to plan and design a new custom themed layout inspired by older pinpall machine scoreboards.
- **Grid idea:**
  - Each dot corresponds to a metric box (CPU, Memory, Disks, Network, Inference, etc.).
  - Arrange dots in rows/columns matching the desired horizontal function. For example, one row per metric and multiple columns for sub‑metrics (e.g., CPU cores vs usage).
  - Use `theme[dot]` variables to fill each dot with a braille character that represents its symbol or color.
- **Design steps:**
  1. Map existing btop boxes onto the grid cells.
  2. Replace default box borders and symbols with braille characters (e.g., `U+2801` for CPU, `U+28A0` for memory).
  3. Define a new theme file `braille-scoreboard.theme` that sets:
     - `main_bg`, `dot_background`, `dot_fg`.
     - Per‑cell variables: `cpu_dot`, `mem_dot`, `disk_dot`, `net_dot`, `inference_dot`.
  4. Create a new preset (e.g., `braille_horizontal`) that shows only the dot grid, with horizontal bars spanning the width of the grid and vertical spacing for each row.
- **Layout constraints:**
  - Keep within ~4 lines vertically; use herdr panes to split rows if needed.
  - Use compact graph symbols (`block` or `tty`) inside braille cells.

## 10. Questions / decisions

1. How many dots (cells) should span the width? Should we match terminal width (~40 columns) or a fixed number of metrics?
2. Which Braille Unicode blocks to prioritize: Braille Patterns (U+2800–U+28FF), Geometric Shapes (U+25A0–U+25FF), Box Drawing/Block Elements (U+2500–U+259F)?
3. Do we want the dot grid as a single horizontal row or multiple rows? If multiple, how many?
4. Should existing btop boxes be fully replaced by braille dots, or used as background with braille characters overlaid?
5. Need to decide if herdr panes should align with grid columns (e.g., one pane per column) or rows.

