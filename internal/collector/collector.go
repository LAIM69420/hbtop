package collector

import (
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	netstat "github.com/shirou/gopsutil/v4/net"
)

type Snapshot struct {
	CPUPercent float64
	MemUsed    uint64
	MemTotal   uint64
	DiskUsed   uint64
	DiskTotal  uint64
	NetRx      float64 // bytes per second
	NetTx      float64 // bytes per second
	At         time.Time
}

type Collector struct {
	lastNetRx   uint64
	lastNetTx   uint64
	lastNetAt   time.Time
	initialized bool
}

func New() *Collector {
	return &Collector{}
}

func (c *Collector) Collect() Snapshot {
	now := time.Now()
	s := Snapshot{At: now}

	if p, err := cpu.Percent(0, false); err == nil && len(p) > 0 {
		s.CPUPercent = p[0]
	}

	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		s.MemUsed = vm.Used
		s.MemTotal = vm.Total
	}

	// Prefer root, fall back to the first usage entry.
	if u, err := disk.Usage("/"); err == nil && u != nil {
		s.DiskUsed = u.Used
		s.DiskTotal = u.Total
	} else if parts, err := disk.Partitions(false); err == nil {
		for _, p := range parts {
			if u, err := disk.Usage(p.Mountpoint); err == nil && u != nil {
				s.DiskUsed = u.Used
				s.DiskTotal = u.Total
				break
			}
		}
	}

	if counters, err := netstat.IOCounters(false); err == nil && len(counters) > 0 {
		rx := counters[0].BytesRecv
		tx := counters[0].BytesSent
		if c.initialized {
			dt := now.Sub(c.lastNetAt).Seconds()
			if dt > 0 {
				s.NetRx = float64(rx-c.lastNetRx) / dt
				s.NetTx = float64(tx-c.lastNetTx) / dt
			}
		}
		c.lastNetRx = rx
		c.lastNetTx = tx
		c.lastNetAt = now
		c.initialized = true
	}

	return s
}
