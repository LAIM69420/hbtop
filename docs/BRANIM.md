# branim animations

hbtop can play animations designed with
[branim](https://github.com/LAIM69420/branim) — a Braille text-animation
designer with a GUI and a headless CLI.

## File format

A `.branim.json` file is a git-diffable JSON document:

```json
{
  "magic": "branim/v1",
  "cell_w": 20,
  "cell_h": 8,
  "fps": 12,
  "frames": [
    ["⠁⠂⠄ ...", "..."],
    ["...", "..."]
  ]
}
```

| Field | Meaning |
| --- | --- |
| `magic` | Must be `branim/v1`. |
| `cell_w` | Frame width in terminal cells. |
| `cell_h` | Frame height in terminal cells. |
| `fps` | Playback rate. |
| `frames` | One array of Braille rows per frame. |

Each row is a string of Braille characters (`U+2800–U+28FF`); each cell carries
a 2×4 dot block.

## Using an animation in hbtop

```toml
[[provider]]
match = "qwen"
wordmark = "QWEN"
palette = ["#615ced", "#a26bff"]
anim = "assets/qwen.branim.json"
```

- The file is loaded at startup.
- hbtop centres the frame in the target area.
- The animation loops at the file's `fps`.
- Colour comes from the provider palette; the branim file is monochrome.

If the file fails to load, hbtop falls back to the built-in wordmark.

## Creating an animation with branim

```bash
# create a blank project
python main.py new hello.branim.json --w 24 --h 8 --fps 12

# generate a procedural loop
python main.py generate -o orbit.branim.json --type orbit --frames 24

# preview in the terminal
python main.py play orbit.branim.json --loops 2
```

Open the GUI with `python main.py --gui` to paint frames directly. See the
branim README for the full CLI.

## Tips for hbtop

- Keep `cell_h` small enough to fit the target view (for example 4–8 cells).
- Leave the outer row/column clear; hbtop may add chip or wordmark elements
  around the frame.
- Use high-contrast dot patterns; palette sweeps will brighten cells over time.
