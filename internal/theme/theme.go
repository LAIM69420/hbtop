package theme

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Theme struct {
	Colors    Colors           `toml:"theme"`
	Layout    Layout           `toml:"layout"`
	Readouts  []Readout        `toml:"readout"`
	Inference InferenceConfig  `toml:"inference"`
	Display   DisplayConfig    `toml:"display"`
	Palette   PaletteConfig    `toml:"palette"`
	GPU       GPUConfig        `toml:"gpu"`
	Providers []ProviderConfig `toml:"provider"`
}

type ProviderConfig struct {
	Match    string   `toml:"match"`
	Wordmark string   `toml:"wordmark"`
	Palette  []string `toml:"palette"`
	Anim     string   `toml:"anim"`
}

type Colors struct {
	Background string `toml:"background"`
	Foreground string `toml:"foreground"`
	Accent     string `toml:"accent"`
	Warning    string `toml:"warning"`
}

type Layout struct {
	Rows    int `toml:"rows"`
	Columns int `toml:"columns"`
}

type Readout struct {
	ID    string `toml:"id"`
	Label string `toml:"label"`
	Row   int    `toml:"row"`
	Col   int    `toml:"col"`
	Width int    `toml:"width"`
	Kind  string `toml:"kind"`
	Unit  string `toml:"unit"`
}

type InferenceConfig struct {
	Enabled    bool              `toml:"enabled"`
	URL        string            `toml:"url"`
	PollMS     int               `toml:"poll_ms"`
	IntervalMS int               `toml:"interval_ms"`
	TimeoutMS  int               `toml:"timeout_ms"`
	Fields     map[string]string `toml:"fields"`
}

type DisplayConfig struct {
	DefaultMode    string `toml:"default_mode"`
	AttractEnabled *bool  `toml:"attract_enabled"`
	AttractIdleMS  int    `toml:"attract_idle_ms"`
	AttractStepMS  int    `toml:"attract_step_ms"`
	ActionMS       int    `toml:"action_ms"`
}

func (d DisplayConfig) AttractOn() bool {
	if d.AttractEnabled == nil {
		return false
	}
	return *d.AttractEnabled
}

type PaletteConfig struct {
	Colors     []string `toml:"colors"`
	CycleMS    int      `toml:"cycle_ms"`
	Brightness float64  `toml:"brightness"`
}

type GPUConfig struct {
	Enabled bool `toml:"enabled"`
	PollMS  int  `toml:"poll_ms"`
}

func DefaultPath() string {
	if p := os.Getenv("HBTOP_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, ".config", "hbtop", "theme.toml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "configs/default.toml"
}

func Load(path string) (*Theme, error) {
	var t Theme
	if _, err := toml.DecodeFile(path, &t); err != nil {
		return nil, err
	}
	applyDefaults(&t)
	return &t, nil
}

func applyDefaults(t *Theme) {
	if t.Layout.Rows <= 0 {
		t.Layout.Rows = 4
	}
	if t.Layout.Columns <= 0 {
		t.Layout.Columns = 80
	}
	for i := range t.Readouts {
		r := &t.Readouts[i]
		if r.Kind == "" {
			r.Kind = "bar"
		}
		if r.Width <= 0 {
			r.Width = 40
		}
	}
	if t.Inference.URL == "" {
		t.Inference.URL = "http://127.0.0.1:8081"
	}
	if t.Inference.PollMS <= 0 {
		if t.Inference.IntervalMS > 0 {
			t.Inference.PollMS = t.Inference.IntervalMS
		} else {
			t.Inference.PollMS = 1000
		}
	}
	if t.Inference.TimeoutMS <= 0 {
		t.Inference.TimeoutMS = 1500
	}
	if t.Display.DefaultMode == "" {
		t.Display.DefaultMode = "dashboard"
	}
	if t.Display.AttractIdleMS <= 0 {
		t.Display.AttractIdleMS = 10000
	}
	if t.Display.AttractStepMS <= 0 {
		t.Display.AttractStepMS = 4000
	}
	if t.Display.ActionMS <= 0 {
		t.Display.ActionMS = 3000
	}
	if len(t.Palette.Colors) == 0 {
		t.Palette.Colors = []string{
			"#ffb000", "#ff2d95", "#00e5ff", "#7dff00", "#ff5e00", "#9b5cff",
		}
	}
	if t.Palette.CycleMS <= 0 {
		t.Palette.CycleMS = 6000
	}
	if t.Palette.Brightness <= 0 {
		t.Palette.Brightness = 1.0
	}
	if t.GPU.PollMS <= 0 {
		t.GPU.PollMS = 1000
	}
	if len(t.Providers) == 0 {
		t.Providers = []ProviderConfig{
			{Match: "lfm", Wordmark: "LFM", Palette: []string{"#00c2ff", "#0077ff"}},
			{Match: "qwen", Wordmark: "QWEN", Palette: []string{"#615ced", "#a26bff"}},
			{Match: "ling", Wordmark: "LING", Palette: []string{"#00d4ff", "#0080ff"}},
		}
	}
	for i := range t.Providers {
		if t.Providers[i].Match == "" {
			t.Providers[i].Match = t.Providers[i].Wordmark
		}
		if t.Providers[i].Wordmark == "" {
			t.Providers[i].Wordmark = "LLM"
		}
		if len(t.Providers[i].Palette) == 0 {
			t.Providers[i].Palette = []string{"#ffb000", "#ff2d95"}
		}
	}
}
