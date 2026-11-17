package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"

	"hbtop/internal/collector"
	"hbtop/internal/inference"
	"hbtop/internal/leaderboard"
	"hbtop/internal/theme"
	"hbtop/internal/ui"
)

func main() {
	configPath := theme.DefaultPath()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help":
			fmt.Println("Usage: hbtop [config.toml]")
			fmt.Println("Env: HBTOP_CONFIG can point at a theme file.")
			return
		case "-v", "--version":
			fmt.Println("hbtop 0.1.0")
			return
		}
		configPath = os.Args[1]
	}

	th, err := theme.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hbtop: failed to load theme %s: %v\n", configPath, err)
		os.Exit(1)
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hbtop: failed to create screen: %v\n", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "hbtop: failed to init screen: %v\n", err)
		os.Exit(1)
	}
	defer screen.Fini()

	lb := leaderboard.Load(leaderboard.DefaultPath())
	app := ui.New(screen, th, lb)
	coll := collector.New()
	gpu := collector.NewGPU()
	procColl := collector.NewProcess()
	tempColl := collector.NewTemp()
	watcher := inference.NewWatcher(th.Inference)

	events := make(chan tcell.Event)
	go func() {
		for {
			events <- screen.PollEvent()
		}
	}()

	stateCh := make(chan inference.State, 64)
	eventCh := make(chan inference.Event, 16)
	gpuCh := make(chan collector.GPUSnapshot, 16)
	procCh := make(chan []collector.ProcessInfo, 16)
	tempCh := make(chan collector.TempSnapshot, 16)

	go func() {
		interval := time.Duration(th.Inference.PollMS) * time.Millisecond
		if interval <= 0 {
			interval = 250 * time.Millisecond
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			st := watcher.Poll()
			select {
			case stateCh <- st:
			default:
			}
		}
	}()

	go func() {
		interval := time.Duration(th.GPU.PollMS) * time.Millisecond
		if interval <= 0 {
			interval = time.Second
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			s := gpu.Collect()
			select {
			case gpuCh <- s:
			default:
			}
		}
	}()

	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for range t.C {
			s := tempColl.Collect()
			select {
			case tempCh <- s:
			default:
			}
		}
	}()

	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for range t.C {
			p := procColl.Collect(200)
			select {
			case procCh <- p:
			default:
			}
		}
	}()

	app.SetSnapshot(coll.Collect())
	app.SetGPU(gpu.Collect())
	app.SetTemps(tempColl.Collect())
	app.SetProcesses(procColl.Collect(200))
	initialState := watcher.Poll()
	drainEvents(watcher.Events())
	app.SetInference(initialState)
	if !initialState.Online {
		app.NotifyEvent(inference.Event{Kind: inference.EventOffline})
	}
	go func() {
		for e := range watcher.Events() {
			eventCh <- e
		}
	}()
	app.Render()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	renderTicker := time.NewTicker(100 * time.Millisecond)
	defer renderTicker.Stop()

	for {
		select {
		case ev := <-events:
			switch ev := ev.(type) {
			case *tcell.EventKey:
				if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
					return
				}
				if ev.Key() == tcell.KeyTab {
					app.CycleCenter()
					continue
				}
				switch ev.Rune() {
				case 'q', 'Q':
					return
				case '1':
					app.SetCenterPP()
				case '2':
					app.SetCenterTG()
				case 'l', 'L':
					app.ToggleLeaderboard()
				case 'i', 'I':
					app.SetCenterLLM()
				case 'p', 'P':
					app.SetCenterProc()
				case 'm', 'M':
					app.CycleLeaderboardMode()
				}
			case *tcell.EventResize:
				screen.Sync()
				app.Render()
			}
		case st := <-stateCh:
			app.SetInference(st)
		case e := <-eventCh:
			app.NotifyEvent(e)
		case s := <-gpuCh:
			app.SetGPU(s)
		case t := <-tempCh:
			app.SetTemps(t)
		case p := <-procCh:
			app.SetProcesses(p)
		case <-ticker.C:
			app.SetSnapshot(coll.Collect())
		case <-renderTicker.C:
			app.Render()
		}
	}
}

func drainEvents(ch <-chan inference.Event) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
