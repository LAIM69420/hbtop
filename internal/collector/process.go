package collector

import (
	"sort"

	"github.com/shirou/gopsutil/v4/process"
)

type ProcessInfo struct {
	PID  int32
	Name string
	User string
	CPU  float64
	Mem  float64
}

type ProcessCollector struct{}

func NewProcess() *ProcessCollector { return &ProcessCollector{} }

func (p *ProcessCollector) Collect(limit int) []ProcessInfo {
	procs, err := process.Processes()
	if err != nil {
		return nil
	}
	out := make([]ProcessInfo, 0, len(procs))
	for _, pr := range procs {
		cpu, _ := pr.CPUPercent()
		mem, _ := pr.MemoryPercent()
		user, _ := pr.Username()
		name, _ := pr.Name()
		out = append(out, ProcessInfo{
			PID:  pr.Pid,
			Name: name,
			User: user,
			CPU:  cpu,
			Mem:  float64(mem),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CPU > out[j].CPU })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
