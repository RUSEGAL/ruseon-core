package main

import (
	"context"
	"os"
	"sync"
	"time"

	gocpu "github.com/shirou/gopsutil/v4/cpu"
	gonet "github.com/shirou/gopsutil/v4/net"
	goproc "github.com/shirou/gopsutil/v4/process"
)

// hwCollector собирает аппаратные метрики в фоне (CPU, RSS, Network I/O).
type hwCollector struct {
	mu      sync.Mutex
	samples []float64 // CPU% snapshots

	netStart     []gonet.IOCountersStat
	netStartTime time.Time

	proc *goproc.Process
}

func newHWCollector() *hwCollector {
	// #nosec G115 -- pid is within 32-bit integer range on supported OS
	p, _ := goproc.NewProcess(int32(os.Getpid()))
	netStats, _ := gonet.IOCounters(false) // false = суммарно по всем интерфейсам

	return &hwCollector{
		proc:         p,
		netStart:     netStats,
		netStartTime: time.Now(),
	}
}

// runSampler запускает фоновый сбор CPU% каждую секунду без блокирующих sleep.
func (h *hwCollector) runSampler(ctx context.Context) {
	// Инициализируем baseline для gopsutil (первый вызов с 0 длительностью инициализирует внутренний таймстамп)
	_, _ = gocpu.Percent(0, false)

	t := time.NewTicker(1 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// Системный CPU% за последнюю секунду (0 = не блокирует, считает дельту с прошлого вызова)
			pcts, err := gocpu.Percent(0, false)
			if err == nil && len(pcts) > 0 {
				h.mu.Lock()
				h.samples = append(h.samples, pcts[0])
				h.mu.Unlock()
			}
		}
	}
}

// result возвращает итоговые метрики.
func (h *hwCollector) result() (cpuAvg, cpuPeak, procCPU float64, rssMB uint64, rxMbps, txMbps float64) {
	// CPU avg + peak из накопленных сэмплов
	h.mu.Lock()
	samples := make([]float64, len(h.samples))
	copy(samples, h.samples)
	h.mu.Unlock()

	if len(samples) > 0 {
		var sum float64
		for _, v := range samples {
			sum += v
			if v > cpuPeak {
				cpuPeak = v
			}
		}
		cpuAvg = sum / float64(len(samples))
	}

	// Процессный CPU% (снимок)
	if h.proc != nil {
		p, err := h.proc.CPUPercent()
		if err == nil {
			procCPU = p
		}
		// RSS
		mem, err := h.proc.MemoryInfo()
		if err == nil && mem != nil {
			rssMB = mem.RSS / 1024 / 1024
		}
	}

	// Network I/O — дельта от начала теста
	netEnd, err := gonet.IOCounters(false)
	if err == nil && len(netEnd) > 0 && len(h.netStart) > 0 {
		elapsed := time.Since(h.netStartTime).Seconds()
		if elapsed > 0 {
			rxBytes := netEnd[0].BytesRecv - h.netStart[0].BytesRecv
			txBytes := netEnd[0].BytesSent - h.netStart[0].BytesSent
			rxMbps = float64(rxBytes*8) / elapsed / 1e6
			txMbps = float64(txBytes*8) / elapsed / 1e6
		}
	}

	return
}
