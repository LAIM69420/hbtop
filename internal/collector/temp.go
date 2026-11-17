package collector

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type TempSnapshot struct {
	CPUTemp float64
	RAMTemp float64
	GPUTemp float64
	HasCPU  bool
	HasRAM  bool
	HasGPU  bool
}

type TempCollector struct{}

func NewTemp() *TempCollector { return &TempCollector{} }

func (t *TempCollector) Collect() TempSnapshot {
	var s TempSnapshot
	dirs, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, dir := range dirs {
		nameBytes, err := os.ReadFile(filepath.Join(dir, "name"))
		if err != nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(string(nameBytes)))
		temp, ok := readTemp(filepath.Join(dir, "temp1_input"))
		if !ok {
			continue
		}
		switch {
		case strings.Contains(name, "k10temp") || strings.Contains(name, "coretemp") || strings.Contains(name, "cpu"):
			s.CPUTemp = temp
			s.HasCPU = true
		case strings.Contains(name, "spd5118") || strings.Contains(name, "dimm") || strings.Contains(name, "ram"):
			s.RAMTemp = temp
			s.HasRAM = true
		case strings.Contains(name, "amdgpu"):
			s.GPUTemp = temp
			s.HasGPU = true
		}
	}
	if !s.HasCPU {
		for _, dir := range dirs {
			nameBytes, _ := os.ReadFile(filepath.Join(dir, "name"))
			name := strings.ToLower(strings.TrimSpace(string(nameBytes)))
			if strings.Contains(name, "acpitz") {
				if temp, ok := readTemp(filepath.Join(dir, "temp1_input")); ok {
					s.CPUTemp = temp
					s.HasCPU = true
					break
				}
			}
		}
	}
	return s
}

func readTemp(path string) (float64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if err != nil {
		return 0, false
	}
	return v / 1000.0, true
}
