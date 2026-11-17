package collector

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
)

type GPUSnapshot struct {
	Available bool
	Name      string
	Usage     float64
	VRAM      float64
	Temp      float64
}

type GPUCollector struct{}

func NewGPU() *GPUCollector { return &GPUCollector{} }

func (g *GPUCollector) Collect() GPUSnapshot {
	cmd := exec.Command("rocm-smi", "--showproductname", "--showuse", "--showmemuse", "--showtemp", "--json")
	out, err := cmd.Output()
	if err != nil {
		return GPUSnapshot{}
	}

	var raw map[string]map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		return GPUSnapshot{}
	}

	card, ok := raw["card0"]
	if !ok {
		for _, v := range raw {
			card = v
			ok = true
			break
		}
	}
	if !ok {
		return GPUSnapshot{}
	}

	return GPUSnapshot{
		Available: true,
		Name:      getString(card, "Card Series"),
		Usage:     getNumber(card, "GPU use (%)"),
		VRAM:      getNumber(card, "GPU Memory Allocated (VRAM%)"),
		Temp:      getNumber(card, "Temperature (Sensor edge) (C)"),
	}
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getNumber(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case string:
		s := strings.TrimSpace(n)
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	return 0
}
