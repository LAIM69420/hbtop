package ui

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"hbtop/internal/anim"
	"hbtop/internal/braille"
	"hbtop/internal/collector"
	"hbtop/internal/inference"
	"hbtop/internal/leaderboard"
	"hbtop/internal/theme"
)

type mode int

const (
	modeDashboard mode = iota
	modeAttract
	modeAction
)

type actionKind int

const (
	actionNone actionKind = iota
	actionLoading
	actionReady
	actionFinished
)

type centerView int

const (
	viewLeaderboardTG centerView = iota
	viewLeaderboardPP
	viewLLM
	viewProcesses
)

type lbMode int

const (
	lbCombined lbMode = iota
	lbModel
	lbRun
)

type rgb struct{ r, g, b float64 }

type rect struct{ x, y, w, h int }

type provider struct {
	match    string
	wordmark string
	palette  []rgb
	anim     *anim.Project
}

type UI struct {
	screen tcell.Screen
	theme  *theme.Theme
	lb     *leaderboard.Store

	snapshot  collector.Snapshot
	gpu       collector.GPUSnapshot
	temps     collector.TempSnapshot
	processes []collector.ProcessInfo
	inference inference.State

	action      actionKind
	actionStart time.Time
	actionUntil time.Time
	actionText  string
	actionSub   string
	lastActive  time.Time
	frame       int

	center     centerView
	returnView centerView
	lbMode     lbMode

	palette       []rgb
	activePalette []rgb
	providers     []provider

	ballX, ballY   float64
	ballVX, ballVY float64
	ballTrail      []point

	cpuHist, gpuHist, memHist, diskHist []float64
	flashCPU, flashGPU, flashMem, flashDisk time.Time

	particles []particle
	transitionStart time.Time

	overrides  []colorZone
	cpuChipZone colorZone
	ramChipZone colorZone
}

type point struct{ x, y float64 }

type particle struct {
	x, y, vx, vy, life float64
}

type colorZone struct {
	x, y, w, h int
	color      tcell.Color
}

func New(screen tcell.Screen, th *theme.Theme, lb *leaderboard.Store) *UI {
	lastActive := time.Now()
	if th.Display.DefaultMode == "attract" {
		idle := time.Duration(th.Display.AttractIdleMS) * time.Millisecond
		if idle <= 0 {
			idle = 10 * time.Second
		}
		lastActive = lastActive.Add(-idle - time.Second)
	}
	u := &UI{
		screen:     screen,
		theme:      th,
		lb:         lb,
		lastActive: lastActive,
		center:     viewLeaderboardTG,
		returnView: viewLeaderboardTG,
	}
	u.palette = parsePalette(th.Palette.Colors)
	if len(u.palette) == 0 {
		u.palette = []rgb{{1, 0.69, 0}, {1, 0.18, 0.58}, {0, 0.9, 1}, {0.49, 1, 0}, {1, 0.37, 0}, {0.61, 0.36, 1}}
	}
	for _, pc := range th.Providers {
		p := provider{
			match:    strings.ToLower(pc.Match),
			wordmark: pc.Wordmark,
			palette:  parsePalette(pc.Palette),
		}
		if pc.Anim != "" {
			if a, err := anim.Load(pc.Anim); err == nil {
				p.anim = a
			}
		}
		u.providers = append(u.providers, p)
	}
	u.ballX = 20
	u.ballY = 10
	u.ballVX = 1.4
	u.ballVY = 0.9
	return u
}

func (u *UI) SetSnapshot(s collector.Snapshot) {
	oldCPU := u.snapshot.CPUPercent
	oldMem := percent(u.snapshot.MemUsed, u.snapshot.MemTotal)
	oldDisk := percent(u.snapshot.DiskUsed, u.snapshot.DiskTotal)
	u.snapshot = s
	mem := percent(s.MemUsed, s.MemTotal)
	disk := percent(s.DiskUsed, s.DiskTotal)
	u.cpuHist = appendHist(u.cpuHist, s.CPUPercent)
	u.memHist = appendHist(u.memHist, mem)
	u.diskHist = appendHist(u.diskHist, disk)
	now := time.Now()
	if math.Abs(s.CPUPercent-oldCPU) > 1.5 {
		u.flashCPU = now.Add(400 * time.Millisecond)
	}
	if math.Abs(mem-oldMem) > 1.0 {
		u.flashMem = now.Add(400 * time.Millisecond)
	}
	if math.Abs(disk-oldDisk) > 0.5 {
		u.flashDisk = now.Add(400 * time.Millisecond)
	}
}

func (u *UI) SetGPU(s collector.GPUSnapshot) {
	old := u.gpu.Usage
	u.gpu = s
	u.gpuHist = appendHist(u.gpuHist, s.Usage)
	if math.Abs(s.Usage-old) > 2.0 {
		u.flashGPU = time.Now().Add(400 * time.Millisecond)
	}
}

func (u *UI) SetTemps(t collector.TempSnapshot) { u.temps = t }
func (u *UI) SetProcesses(p []collector.ProcessInfo) { u.processes = p }
func (u *UI) SetInference(s inference.State)         { u.inference = s }

func appendHist(h []float64, v float64) []float64 {
	h = append(h, v)
	if len(h) > 64 {
		h = h[len(h)-64:]
	}
	return h
}

func (u *UI) SetCenterPP()   { u.center = viewLeaderboardPP; u.touch() }
func (u *UI) SetCenterTG()   { u.center = viewLeaderboardTG; u.touch() }
func (u *UI) SetCenterLLM()  { u.center = viewLLM; u.touch() }
func (u *UI) SetCenterProc() { u.center = viewProcesses; u.touch() }

func (u *UI) touch() {
	u.lastActive = time.Now()
	u.transitionStart = time.Now()
}

func (u *UI) ToggleLeaderboard() {
	if u.center == viewLeaderboardPP {
		u.center = viewLeaderboardTG
	} else {
		u.center = viewLeaderboardPP
	}
	u.touch()
}

func (u *UI) CycleCenter() {
	switch u.center {
	case viewLeaderboardTG:
		u.center = viewLeaderboardPP
	case viewLeaderboardPP:
		u.center = viewLLM
	case viewLLM:
		u.center = viewProcesses
	default:
		u.center = viewLeaderboardTG
	}
	u.touch()
}

func (u *UI) CycleLeaderboardMode() {
	switch u.lbMode {
	case lbCombined:
		u.lbMode = lbModel
	case lbModel:
		u.lbMode = lbRun
	default:
		u.lbMode = lbCombined
	}
	u.touch()
}

func (u *UI) providerFor(model string) *provider {
	lower := strings.ToLower(model)
	for i := range u.providers {
		if u.providers[i].match != "" && strings.Contains(lower, u.providers[i].match) {
			return &u.providers[i]
		}
	}
	return nil
}

func (u *UI) attractProvider() *provider {
	if len(u.providers) == 0 {
		return nil
	}
	step := u.theme.Display.AttractStepMS
	if step <= 0 {
		step = 4000
	}
	idx := int(time.Now().UnixMilli()/int64(step)) % len(u.providers)
	return &u.providers[idx]
}

func (u *UI) NotifyEvent(e inference.Event) {
	u.lastActive = time.Now()
	d := time.Duration(u.theme.Display.ActionMS) * time.Millisecond
	if d <= 0 {
		d = 3 * time.Second
	}
	switch e.Kind {
	case inference.EventOffline:
		u.action = actionLoading
		u.actionStart = time.Now()
		u.actionUntil = time.Now().Add(30 * time.Second)
		u.actionText = "LOADING"
		u.actionSub = "MODEL"
	case inference.EventModelReady:
		u.action = actionReady
		u.actionStart = time.Now()
		u.actionUntil = time.Now().Add(d)
		u.actionText = "READY"
		u.actionSub = shortModel(e.Model)
	case inference.EventInferenceStarted:
		u.returnView = u.center
		u.center = viewLLM
	case inference.EventInferenceFinished:
		ppRank, tgRank := 0, 0
		if u.lb != nil {
			ppRank, tgRank = u.lb.AddRun(e.Model, e.PromptTPS, e.PredictedTPS)
		}
		u.center = u.returnView
		u.action = actionFinished
		u.actionStart = time.Now()
		u.actionUntil = time.Now().Add(d)
		switch {
		case tgRank == 1:
			u.actionText = "NEW #1 TG"
			u.actionSub = fmt.Sprintf("%.0f %s", e.PredictedTPS, shortModel(e.Model))
		case ppRank == 1:
			u.actionText = "NEW #1 PP"
			u.actionSub = fmt.Sprintf("%.0f %s", e.PromptTPS, shortModel(e.Model))
		default:
			u.actionText = "JACKPOT"
			u.actionSub = fmt.Sprintf("%.0f T/S", e.PredictedTPS)
		}
	}
}

func (u *UI) Render() {
	u.frame++
	w, h := u.screen.Size()
	u.screen.Clear()
	if w <= 0 || h <= 0 {
		u.screen.Show()
		return
	}

	cw, ch := w, h
	canvas := braille.New(cw, ch)
	dotW, dotH := cw*2, ch*4

	now := time.Now()
	actionActive := u.action != actionNone && now.Before(u.actionUntil)
	if u.action != actionNone && !actionActive {
		u.action = actionNone
	}
	u.activePalette = nil
	u.overrides = nil
	full := rect{0, 0, dotW, dotH}
	offline := !u.inference.Online

	switch {
	case offline:
		p := u.attractProvider()
		if p != nil {
			u.activePalette = p.palette
		}
		u.drawLogoAttract(canvas, full, p)
	case actionActive:
		if u.action == actionReady {
			if p := u.providerFor(u.inference.Model); p != nil {
				u.activePalette = p.palette
			}
		}
		u.drawAction(canvas, full)
	case u.theme.Display.AttractOn() && now.Sub(u.lastActive) > time.Duration(u.theme.Display.AttractIdleMS)*time.Millisecond:
		u.drawAttract(canvas, full)
	default:
		if u.center == viewLLM {
			if p := u.providerFor(u.inference.Model); p != nil {
				u.activePalette = p.palette
			}
		}
		u.drawDashboard(canvas, full)
	}

	for ry := 0; ry < ch; ry++ {
		runes := canvas.RenderRow(ry)
		for cx := 0; cx < cw; cx++ {
			r := ' '
			if cx < len(runes) {
				r = runes[cx]
			}
			style := tcell.StyleDefault.Foreground(u.colorFor(cx, ry))
			u.screen.SetContent(cx, ry, r, nil, style)
		}
	}
	if u.center == viewProcesses && !actionActive && !offline {
		u.drawProcessesASCII()
	}
	u.screen.Show()
}

func (u *UI) drawLogoAttract(c *braille.Canvas, r rect, p *provider) {
	step := u.theme.Display.AttractStepMS
	if step <= 0 {
		step = 4000
	}
	t := float64(time.Now().UnixMilli()%int64(step)) / float64(step)
	progress := math.Min(1, t/0.25)
	effect := 0
	if p != nil {
		for i := range u.providers {
			if &u.providers[i] == p {
				effect = i % 3
				break
			}
		}
	}
	wr := rect{r.x, r.y + 2, r.w, r.h - 14}
	u.drawWordmark(c, wr, p, progress, effect)

	f := braille.SmallFont
	msg := "NO MODEL"
	mw := f.TextWidth(msg, 1)
	f.Draw(c, r.x+(r.w-mw)/2, r.y+r.h-f.Height-2, msg, 1)
}

func (u *UI) drawWordmark(c *braille.Canvas, r rect, p *provider, progress float64, effect int) {
	tmp := braille.New(c.Width, c.Height)
	if p != nil && p.anim != nil {
		frame := (u.frame * p.anim.FPS / 10) % p.anim.Len()
		ax := r.x + (r.w-p.anim.CellW*2)/2
		ay := r.y + (r.h-p.anim.CellH*4)/2
		p.anim.Draw(tmp, frame, ax, ay)
	} else {
		text := "LLM"
		if p != nil {
			text = p.wordmark
		} else if u.inference.Model != "" {
			text = fallbackWordmark(u.inference.Model)
		}
		f := braille.SmallFont
		scale := f.FitScale(text, r.w-8, r.h-4, 3)
		tw := f.TextWidth(text, scale)
		th := f.Height * scale
		tx := r.x + (r.w-tw)/2
		ty := r.y + (r.h-th)/2
		if effect == 2 {
			bounce := math.Abs(math.Sin(progress*math.Pi)) * float64(r.h) / 6
			ty -= int(bounce * (1 - progress))
		}
		f.Draw(tmp, tx, ty, text, scale)
	}

	for y := r.y; y < r.y+r.h; y++ {
		for x := r.x; x < r.x+r.w; x++ {
			if !tmp.GetDot(x, y) {
				continue
			}
			if wordmarkVisible(x, y, r, progress, effect) {
				c.SetDot(x, y)
			}
		}
	}
}

func wordmarkVisible(x, y int, r rect, p float64, effect int) bool {
	if p >= 1 {
		return true
	}
	if effect == 1 {
		return float64(y-r.y) <= p*float64(r.h)
	}
	h := (x*73856093 ^ y*19349663) & 0xffff
	return float64(h)/65535.0 < p
}

func (u *UI) drawChaseBorder(c *braille.Canvas, r rect) {
	phase := u.frame
	for x := r.x; x < r.x+r.w; x += 4 {
		if (x/4+phase)%4 == 0 {
			c.SetDot(x, r.y)
			c.SetDot(x, r.y+r.h-1)
		}
	}
	for y := r.y; y < r.y+r.h; y += 4 {
		if (y/4+phase)%4 == 0 {
			c.SetDot(r.x, y)
			c.SetDot(r.x+r.w-1, y)
		}
	}
}

func (u *UI) updateBall(w, h int) {
	u.ballTrail = append(u.ballTrail, point{u.ballX, u.ballY})
	if len(u.ballTrail) > 6 {
		u.ballTrail = u.ballTrail[1:]
	}
	speed := 1.0 + u.snapshot.CPUPercent/200
	u.ballX += u.ballVX * speed
	u.ballY += u.ballVY * speed
	if u.ballX < 2 {
		u.ballX = 2
		u.ballVX = math.Abs(u.ballVX)
	}
	if u.ballX > float64(w-2) {
		u.ballX = float64(w - 2)
		u.ballVX = -math.Abs(u.ballVX)
	}
	if u.ballY < 2 {
		u.ballY = 2
		u.ballVY = math.Abs(u.ballVY)
	}
	if u.ballY > float64(h-2) {
		u.ballY = float64(h - 2)
		u.ballVY = -math.Abs(u.ballVY)
	}
}

func (u *UI) drawBall(c *braille.Canvas) {
	for i, p := range u.ballTrail {
		if i%2 != 0 {
			continue
		}
		c.SetDot(int(p.x), int(p.y))
	}
	x := int(u.ballX)
	y := int(u.ballY)
	for dy := 0; dy < 2; dy++ {
		for dx := 0; dx < 2; dx++ {
			c.SetDot(x+dx, y+dy)
		}
	}
}

func (u *UI) burstCenter(n int, speed float64) {
	cw, ch := u.screen.Size()
	u.spawnBurst(cw, ch*2, n, speed)
}

func (u *UI) spawnBurst(cx, cy, n int, speed float64) {
	for i := 0; i < n; i++ {
		ang := float64(i)*2*math.Pi/float64(n) + float64(u.frame)*0.13
		sp := speed * (0.5 + 0.5*math.Abs(math.Sin(float64(i)*1.7)))
		u.particles = append(u.particles, particle{
			x:    float64(cx),
			y:    float64(cy),
			vx:   math.Cos(ang) * sp,
			vy:   math.Sin(ang) * sp,
			life: 1.0,
		})
	}
}

func (u *UI) updateParticles() {
	alive := u.particles[:0]
	for _, p := range u.particles {
		p.x += p.vx
		p.y += p.vy
		p.vy += 0.12
		p.life -= 0.035
		if p.life > 0 {
			alive = append(alive, p)
		}
	}
	u.particles = alive
}

func (u *UI) drawParticles(c *braille.Canvas) {
	for _, p := range u.particles {
		c.SetDot(int(p.x), int(p.y))
		if p.life > 0.6 {
			c.SetDot(int(p.x)+1, int(p.y))
		}
	}
}

func (u *UI) drawTwinkles(c *braille.Canvas, w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	for i := 0; i < 12; i++ {
		seed := uint64(i*2654435761 + (u.frame/4)*40503)
		x := int(seed % uint64(w))
		y := int((seed >> 16) % uint64(h))
		c.SetDot(x, y)
	}
}

func (u *UI) drawTransition(c *braille.Canvas, w, h int) {
	if u.transitionStart.IsZero() {
		return
	}
	d := time.Since(u.transitionStart)
	if d < 0 || d > 350*time.Millisecond {
		return
	}
	p := float64(d) / float64(350*time.Millisecond)
	x := int(p * float64(w))
	for y := 0; y < h; y++ {
		c.SetDot(x, y)
		c.SetDot(x+1, y)
	}
}

func (u *UI) drawDashboard(c *braille.Canvas, r rect) {
	leftW := int(float64(r.w) * 0.30)
	rightW := int(float64(r.w) * 0.30)
	centerW := r.w - leftW - rightW

	left := rect{r.x, r.y, leftW, r.h}
	center := rect{r.x + leftW, r.y, centerW, r.h}
	right := rect{r.x + leftW + centerW, r.y, rightW, r.h}

	statusH := 8
	if r.h < 44 {
		statusH = 0
	}
	chipH := r.h - statusH
	if chipH < 12 {
		chipH = r.h
		statusH = 0
	}

	u.drawCPU(c, rect{left.x, left.y, left.w, chipH})
	if statusH > 0 {
		u.drawStatusLine(c, rect{left.x, left.y + chipH, left.w, statusH}, "GPU", u.gpuValue())
	}

	u.drawRAM(c, rect{right.x, right.y, right.w, chipH})
	if statusH > 0 {
		u.drawStatusLine(c, rect{right.x, right.y + chipH, right.w, statusH}, "DSK", fmt.Sprintf("%.0f%%", percent(u.snapshot.DiskUsed, u.snapshot.DiskTotal)))
	}

	switch u.center {
	case viewLLM:
		u.drawLLM(c, center)
	case viewLeaderboardPP:
		u.drawLeaderboard(c, center, "PP")
	case viewProcesses:
		// ASCII overlay is drawn after the canvas blit.
	default:
		u.drawLeaderboard(c, center, "TG")
	}
}

func (u *UI) drawStatusLine(c *braille.Canvas, r rect, label, value string) {
	f := braille.SmallFont
	text := fmt.Sprintf("%s %s", label, value)
	tw := f.TextWidth(text, 1)
	f.Draw(c, r.x+(r.w-tw)/2, r.y+(r.h-f.Height)/2, text, 1)
}

func (u *UI) drawCPU(c *braille.Canvas, r rect) {
	color := tempColor(u.temps.CPUTemp, u.snapshot.CPUPercent, u.temps.HasCPU)
	u.cpuChipZone = cellZone(r, color)
	u.overrides = append(u.overrides, u.cpuChipZone)

	if r.h < 32 {
		u.drawCPUCompact(c, r)
		return
	}

	size := minInt(r.w-8, r.h-8)
	if size < 16 {
		size = minInt(r.w, r.h) - 2
	}
	if size < 8 {
		size = 8
	}
	size -= size % 2
	cx := r.x + (r.w-size)/2
	cy := r.y + (r.h-size)/2
	drawRect(c, cx, cy, size, size)
	drawRect(c, cx+3, cy+3, size-6, size-6)
	for i := 0; i < size; i += 4 {
		drawLine(c, cx+i, cy-3, cx+i, cy-1)
		drawLine(c, cx+i, cy+size, cx+i, cy+size+2)
		drawLine(c, cx-3, cy+i, cx-1, cy+i)
		drawLine(c, cx+size, cy+i, cx+size+2, cy+i)
	}

	f := braille.SmallFont
	innerX := cx + 3
	innerY := cy + 3
	innerW := size - 6
	innerH := size - 6
	drawNumberPct(c, innerX, innerY+2, innerW, innerH/2, u.snapshot.CPUPercent, f)
	temp := "??"
	if u.temps.HasCPU {
		temp = fmt.Sprintf("%.0f°", u.temps.CPUTemp)
	}
	drawCenteredText(c, innerX, innerY+innerH/2+2, innerW, innerH-innerH/2-2, temp, f, 2)
}

func (u *UI) drawRAM(c *braille.Canvas, r rect) {
	color := tempColor(u.temps.RAMTemp, percent(u.snapshot.MemUsed, u.snapshot.MemTotal), u.temps.HasRAM)
	u.ramChipZone = cellZone(r, color)
	u.overrides = append(u.overrides, u.ramChipZone)

	if r.h < 32 {
		u.drawRAMCompact(c, r)
		return
	}

	w := r.w - 4
	if w < 20 {
		w = r.w - 2
	}
	h := 40
	if h > r.h-6 {
		h = r.h - 6
	}
	if h < 16 {
		h = 16
	}
	x := r.x + (r.w-w)/2
	y := r.y + (r.h-h)/2

	drawRect(c, x, y, w, h)
	// Memory packages along the top.
	chipW := (w - 8) / 3
	if chipW > 10 {
		chipW = 10
	}
	if chipW < 4 {
		chipW = 4
	}
	for i := 0; i < 3; i++ {
		cx := x + 4 + i*(chipW+3)
		drawRect(c, cx, y+2, chipW, 4)
	}
	// Gold contacts along the bottom.
	for i := 0; i < w; i += 3 {
		drawLine(c, x+i, y+h-1, x+i, y+h+2)
	}
	drawRect(c, x+w/2-3, y+h-1, 6, 3)

	f := braille.SmallFont
	statusY := y + 7
	statusH := h - 12
	drawNumberPct(c, x+2, statusY, w-4, statusH/2, percent(u.snapshot.MemUsed, u.snapshot.MemTotal), f)
	temp := "??"
	if u.temps.HasRAM {
		temp = fmt.Sprintf("%.0f°", u.temps.RAMTemp)
	}
	drawCenteredText(c, x+2, statusY+statusH/2, w-4, statusH-statusH/2, temp, f, 2)
}

func (u *UI) drawCPUCompact(c *braille.Canvas, r rect) {
	if r.h <= 16 {
		drawRect(c, r.x, r.y, r.w, r.h)
		temp := ""
		if u.temps.HasCPU {
			temp = fmt.Sprintf("%.0f°", u.temps.CPUTemp)
		}
		drawChipStatusCompact(c, r.x+1, r.y+1, r.w-2, r.h-2, u.snapshot.CPUPercent, temp, braille.SmallFont)
		return
	}
	bx := r.x + 1
	by := r.y + 1
	bw := r.w - 2
	bh := r.h - 2
	if bw < 8 || bh < 8 {
		return
	}
	drawRect(c, bx, by, bw, bh)
	for i := 3; i < bw-2; i += 4 {
		drawLine(c, bx+i, by-2, bx+i, by-1)
		drawLine(c, bx+i, by+bh, bx+i, by+bh+1)
	}
	for j := 3; j < bh-2; j += 4 {
		drawLine(c, bx-2, by+j, bx-1, by+j)
		drawLine(c, bx+bw, by+j, bx+bw+1, by+j)
	}
	temp := ""
	if u.temps.HasCPU {
		temp = fmt.Sprintf("%.0f°", u.temps.CPUTemp)
	}
	drawChipStatusCompact(c, bx+2, by+2, bw-4, bh-4, u.snapshot.CPUPercent, temp, braille.SmallFont)
}

func (u *UI) drawRAMCompact(c *braille.Canvas, r rect) {
	if r.h <= 16 {
		drawRect(c, r.x, r.y, r.w, r.h)
		drawRect(c, r.x+r.w/2-3, r.y+r.h-2, 6, 2)
		temp := ""
		if u.temps.HasRAM {
			temp = fmt.Sprintf("%.0f°", u.temps.RAMTemp)
		}
		drawChipStatusCompact(c, r.x+1, r.y+1, r.w-2, r.h-2, percent(u.snapshot.MemUsed, u.snapshot.MemTotal), temp, braille.SmallFont)
		return
	}
	bx := r.x + 1
	by := r.y + 1
	bw := r.w - 2
	bh := r.h - 2
	if bw < 8 || bh < 8 {
		return
	}
	drawRect(c, bx, by, bw, bh)
	for i := 3; i < bw-2; i += 3 {
		drawLine(c, bx+i, by+bh-1, bx+i, by+bh+1)
	}
	drawRect(c, bx+bw/2-3, by+bh-1, 6, 2)
	drawLine(c, bx+3, by+2, bx+bw-4, by+2)
	temp := ""
	if u.temps.HasRAM {
		temp = fmt.Sprintf("%.0f°", u.temps.RAMTemp)
	}
	drawChipStatusCompact(c, bx+2, by+2, bw-4, bh-4, percent(u.snapshot.MemUsed, u.snapshot.MemTotal), temp, braille.SmallFont)
}

func drawChipStatusCompact(c *braille.Canvas, x, y, w, h int, usage float64, tempText string, f *braille.Font) {
	num := fmt.Sprintf("%.0f", usage)
	pct := "%"
	scale := 2
	if f.Height*scale > h {
		scale = 1
	}
	numW := f.TextWidth(num, scale)
	pctW := f.TextWidth(pct, 1)
	gap := 2
	tempW := 0
	if tempText != "" {
		tempW = f.TextWidth(tempText, 1)
	}
	total := numW + gap + pctW
	if tempW > 0 {
		total += gap + tempW
	}
	if total > w && tempW > 0 {
		tempText = ""
		tempW = 0
		total = numW + gap + pctW
	}
	if total > w {
		pct = ""
		pctW = 0
		total = numW
	}
	startX := x + (w-total)/2
	if startX < x {
		startX = x
	}
	numH := f.Height * scale
	numY := y + (h-numH)/2
	f.Draw(c, startX, numY, num, scale)
	if pct != "" {
		f.Draw(c, startX+numW+gap, numY+numH-f.Height, pct, 1)
	}
	if tempText != "" {
		f.Draw(c, startX+numW+gap+pctW+gap, numY+numH-f.Height, tempText, 1)
	}
}

func drawNumberPct(c *braille.Canvas, x, y, w, h int, value float64, f *braille.Font) {
	num := fmt.Sprintf("%.0f", value)
	scale := f.FitScale(num, w-8, h, 1)
	numW := f.TextWidth(num, scale)
	pctW := f.TextWidth("%", 1)
	gap := 2
	total := numW + gap + pctW
	startX := x + (w-total)/2
	if startX < x {
		startX = x
	}
	numH := f.Height * scale
	numY := y + (h-numH)/2
	f.Draw(c, startX, numY, num, scale)
	f.Draw(c, startX+numW+gap, numY+numH-f.Height, "%", 1)
}

func drawCenteredText(c *braille.Canvas, x, y, w, h int, text string, f *braille.Font, maxScale int) {
	scale := f.FitScale(text, w, h, 1)
	if scale > maxScale {
		scale = maxScale
	}
	tw := f.TextWidth(text, scale)
	th := f.Height * scale
	f.Draw(c, x+(w-tw)/2, y+(h-th)/2, text, scale)
}

func drawRect(c *braille.Canvas, x, y, w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	for i := 0; i < w; i++ {
		c.SetDot(x+i, y)
		c.SetDot(x+i, y+h-1)
	}
	for j := 0; j < h; j++ {
		c.SetDot(x, y+j)
		c.SetDot(x+w-1, y+j)
	}
}

func drawLine(c *braille.Canvas, x0, y0, x1, y1 int) {
	dx := absInt(x1 - x0)
	dy := -absInt(y1 - y0)
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		c.SetDot(x0, y0)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func cellZone(r rect, color tcell.Color) colorZone {
	return colorZone{x: r.x / 2, y: r.y / 4, w: r.w / 2, h: r.h / 4, color: color}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func tempColor(temp, usage float64, hasTemp bool) tcell.Color {
	if !hasTemp || temp <= 0 {
		temp = 35 + usage*0.55
	}
	if temp < 30 {
		temp = 30
	}
	if temp > 90 {
		temp = 90
	}
	cyan := rgb{0, 1, 1}
	yellow := rgb{1, 1, 0}
	orange := rgb{1, 0.5, 0}
	red := rgb{1, 0, 0}
	var c rgb
	switch {
	case temp < 50:
		c = mixRGB(cyan, yellow, (temp-30)/20)
	case temp < 70:
		c = mixRGB(yellow, orange, (temp-50)/20)
	default:
		c = mixRGB(orange, red, (temp-70)/20)
	}
	return tcell.NewRGBColor(int32(c.r*255), int32(c.g*255), int32(c.b*255))
}

func mixRGB(a, b rgb, t float64) rgb {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return rgb{
		r: a.r + (b.r-a.r)*t,
		g: a.g + (b.g-a.g)*t,
		b: a.b + (b.b-a.b)*t,
	}
}

func (u *UI) drawCompact(c *braille.Canvas, r rect, label, value string, hist []float64, max float64, flash time.Time) {
	f := braille.SmallFont
	lw := f.TextWidth(label, 1)
	vw := f.TextWidth(value, 1)
	gap := 3
	total := lw + gap + vw
	startX := r.x + (r.w-total)/2
	if startX < r.x+1 {
		startX = r.x + 1
	}
	y := r.y + 1
	if r.h < 14 {
		y = r.y + (r.h-f.Height)/2
	}
	f.Draw(c, startX, y, label, 1)
	f.Draw(c, startX+lw+gap, y, value, 1)
	if r.h >= 14 {
		u.drawSparkline(c, rect{r.x + 2, r.y + r.h - 6, r.w - 4, 4}, hist, max)
	}
	u.drawFlash(c, r, flash)
}

func (u *UI) drawSparkline(c *braille.Canvas, r rect, hist []float64, max float64) {
	if len(hist) < 2 || max <= 0 || r.w < 4 || r.h < 2 {
		return
	}
	step := 2
	slots := r.w / step
	if slots < 2 {
		return
	}
	start := 0
	if len(hist) > slots {
		start = len(hist) - slots
	}
	for i := start; i < len(hist); i++ {
		x := r.x + (i-start)*step
		v := hist[i] / max
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		y := r.y + r.h - 1 - int(v*float64(r.h-1))
		for yy := y; yy < r.y+r.h; yy += 2 {
			c.SetDot(x, yy)
		}
	}
}

func (u *UI) drawFlash(c *braille.Canvas, r rect, until time.Time) {
	if until.IsZero() || !time.Now().Before(until) {
		return
	}
	for x := r.x; x < r.x+r.w; x += 2 {
		c.SetDot(x, r.y)
		c.SetDot(x, r.y+r.h-1)
	}
	for y := r.y; y < r.y+r.h; y += 2 {
		c.SetDot(r.x, y)
		c.SetDot(r.x+r.w-1, y)
	}
}

func (u *UI) drawWide(c *braille.Canvas, r rect, label, value string) {
	f := braille.SmallFont
	lw := f.TextWidth(label, 1)
	f.Draw(c, r.x+(r.w-lw)/2, r.y+2, label, 1)

	availH := r.h - f.Height - 4
	scale := f.FitScale(value, r.w-4, availH, 2)
	vw := f.TextWidth(value, scale)
	vh := f.Height * scale
	vx := r.x + (r.w-vw)/2
	vy := r.y + f.Height + 4 + (availH-vh)/2
	f.Draw(c, vx, vy, value, scale)
}

func (u *UI) drawLLM(c *braille.Canvas, r rect) {
	f := braille.SmallFont
	model := shortModel(u.inference.Model)
	if model == "" {
		model = "LLM"
	}
	model = fitString(f, model, r.w-4, 1)
	mw := f.TextWidth(model, 1)
	modelY := r.y
	if r.h >= 24 {
		wr := rect{r.x, r.y, r.w, f.Height + 1}
		u.drawWordmark(c, wr, u.providerFor(u.inference.Model), 1, 0)
		modelY = r.y + f.Height + 1
	}
	f.Draw(c, r.x+(r.w-mw)/2, modelY, model, 1)

	pp := u.inference.LastRunPromptTPS
	tg := u.inference.LastRunPredictedTPS
	if u.inference.IsProcessing {
		if u.inference.PromptTPS > 0 {
			pp = u.inference.PromptTPS
		}
		if u.inference.PredictedTPS > 0 {
			tg = u.inference.PredictedTPS
		}
	}
	line := fmt.Sprintf("PP %.0f  TG %.0f", pp, tg)
	if !u.inference.Online {
		line = "OFF"
	}
	lineY := modelY + f.Height + 1
	availH := r.h - (lineY - r.y)
	if availH < f.Height {
		availH = f.Height
	}
	scale := f.FitScale(line, r.w-4, availH, 1)
	vw := f.TextWidth(line, scale)
	vh := f.Height * scale
	vx := r.x + (r.w-vw)/2
	vy := lineY + (availH-vh)/2
	f.Draw(c, vx, vy, line, scale)
}

func (u *UI) drawLeaderboard(c *braille.Canvas, r rect, kind string) {
	entries := u.leaderboardEntries(kind)
	f := braille.SmallFont

	compact := r.h < 22
	top := r.y + 1
	if !compact {
		title := kind + " HIGH SCORES"
		tw := f.TextWidth(title, 1)
		f.Draw(c, r.x+(r.w-tw)/2, r.y+1, title, 1)
		top = r.y + f.Height + 3
	}
	viewH := r.h - (top - r.y)
	reserve := 0
	if u.inference.Online && u.inference.Model != "" {
		reserve = f.Height + 1
		viewH -= reserve
	}
	if viewH <= 0 {
		return
	}

	if compact {
		lineH := 8
		lines := viewH / lineH
		if lines < 1 {
			lines = 1
		}
		maxStart := len(entries) - lines
		if maxStart < 1 {
			maxStart = 1
		}
		start := 0
		if len(entries) > 0 {
			start = int(time.Now().UnixMilli()/2000) % maxStart
		}
		for i := 0; i < lines; i++ {
			idx := start + i
			if idx >= len(entries) {
				break
			}
			e := entries[idx]
			line := fmt.Sprintf("%2d %s %-8s %4.0f", idx+1, e.Tag, truncate(shortModel(e.Model), 8), e.Score)
			f.Draw(c, r.x+1, top+i*lineH, line, 1)
		}
		u.drawModelFooter(c, r, reserve, f)
		return
	}

	// Continuous vertical scrolling, clipped to the center panel.
	c.SetClip(r.x, top, r.w, viewH)
	lineH := 8
	total := len(entries) * lineH
	if total > 0 {
		offset := (u.frame / 2) % total
		for pass := 0; pass < 2; pass++ {
			base := top - offset + pass*total
			for i, e := range entries {
				y := base + i*lineH
				line := fmt.Sprintf("%2d %s %-10s %5.0f", i+1, e.Tag, truncate(shortModel(e.Model), 10), e.Score)
				f.Draw(c, r.x+2, y, line, 1)
			}
		}
	}
	c.ClearClip()
	u.drawModelFooter(c, r, reserve, f)
}

func (u *UI) drawModelFooter(c *braille.Canvas, r rect, reserve int, f *braille.Font) {
	if reserve <= 0 || u.inference.Model == "" {
		return
	}
	name := shortModel(u.inference.Model)
	name = fitString(f, name, r.w-4, 1)
	nw := f.TextWidth(name, 1)
	f.Draw(c, r.x+(r.w-nw)/2, r.y+r.h-f.Height-1, name, 1)
}

func (u *UI) leaderboardEntries(kind string) []leaderboard.Ranked {
	if u.lb == nil {
		return nil
	}
	if kind == "PP" {
		switch u.lbMode {
		case lbModel:
			return rankedFrom(u.lb.PPModel(), "M")
		case lbRun:
			return rankedFrom(u.lb.PPRun(), "R")
		default:
			return u.lb.CombinedPP()
		}
	}
	switch u.lbMode {
	case lbModel:
		return rankedFrom(u.lb.TGModel(), "M")
	case lbRun:
		return rankedFrom(u.lb.TGRun(), "R")
	default:
		return u.lb.CombinedTG()
	}
}

func rankedFrom(entries []leaderboard.Entry, tag string) []leaderboard.Ranked {
	out := make([]leaderboard.Ranked, 0, len(entries))
	for _, e := range entries {
		out = append(out, leaderboard.Ranked{Model: e.Model, Score: e.Score, Tag: tag})
	}
	return out
}

func (u *UI) drawAttract(c *braille.Canvas, r rect) {
	items := u.attractItems()
	step := u.theme.Display.AttractStepMS
	if step <= 0 {
		step = 4000
	}
	idx := int(time.Now().UnixMilli()/int64(step)) % len(items)
	it := items[idx]

	f := braille.SmallFont
	lw := f.TextWidth(it.label, 1)
	f.Draw(c, r.x+(r.w-lw)/2, r.y+4, it.label, 1)

	scale := f.FitScale(it.value, r.w-8, r.h-f.Height-8, 4)
	vw := f.TextWidth(it.value, scale)
	vh := f.Height * scale
	vx := r.x + (r.w-vw)/2
	vy := r.y + f.Height + 8 + (r.h-f.Height-8-vh)/2
	f.Draw(c, vx, vy, it.value, scale)
}

func (u *UI) drawAction(c *braille.Canvas, r rect) {
	if u.action == actionReady {
		p := u.providerFor(u.inference.Model)
		progress := 1.0
		if !u.actionStart.IsZero() {
			progress = math.Min(1, time.Since(u.actionStart).Seconds()/0.8)
		}
		effect := 0
		if p != nil {
			for i := range u.providers {
				if &u.providers[i] == p {
					effect = i % 3
					break
				}
			}
		}
		wr := rect{r.x, r.y + 2, r.w, r.h - 12}
		u.drawWordmark(c, wr, p, progress, effect)
		u.drawStarburst(c, r, 0.5, 0.4)
		f := braille.SmallFont
		msg := "READY"
		mw := f.TextWidth(msg, 1)
		f.Draw(c, r.x+(r.w-mw)/2, r.y+r.h-f.Height-2, msg, 1)
		return
	}

	switch u.action {
	case actionLoading:
		u.drawChase(c, r)
	case actionFinished:
		u.drawStarburst(c, r, 0.5, 0.45)
	}

	text := u.actionText
	sub := u.actionSub
	if text == "" {
		return
	}

	f := braille.SmallFont
	scale := f.FitScale(text, r.w-8, r.h-8, 4)
	vw := f.TextWidth(text, scale)
	vh := f.Height * scale
	vx := r.x + (r.w-vw)/2
	vy := r.y + (r.h-vh)/2
	f.Draw(c, vx, vy, text, scale)

	if sub != "" {
		sw := f.TextWidth(sub, 1)
		f.Draw(c, r.x+(r.w-sw)/2, r.y+r.h-f.Height-3, sub, 1)
	}
}

func (u *UI) attractItems() []struct{ label, value string } {
	return []struct{ label, value string }{
		{"CPU", fmt.Sprintf("%.0f%%", u.snapshot.CPUPercent)},
		{"GPU", u.gpuValue()},
		{"MEM", fmt.Sprintf("%.0f%%", percent(u.snapshot.MemUsed, u.snapshot.MemTotal))},
		{"DSK", fmt.Sprintf("%.0f%%", percent(u.snapshot.DiskUsed, u.snapshot.DiskTotal))},
		{"NET", compactRate(u.snapshot.NetRx + u.snapshot.NetTx)},
		{"LLM", u.llmValue()},
		{"TIME", time.Now().Format("15:04")},
	}
}

func (u *UI) gpuValue() string {
	if !u.gpu.Available {
		return "--"
	}
	return fmt.Sprintf("%.0f%%", u.gpu.Usage)
}

func (u *UI) llmValue() string {
	if !u.inference.Online {
		return "OFF"
	}
	if u.inference.IsProcessing {
		return fmt.Sprintf("%.0f", u.inference.PredictedTPS)
	}
	if u.inference.PredictedTPS > 0 {
		return fmt.Sprintf("%.0f", u.inference.PredictedTPS)
	}
	return "IDLE"
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

func compactRate(bps float64) string {
	switch {
	case bps >= 1024*1024*1024:
		return fmt.Sprintf("%.1fG", bps/(1024*1024*1024))
	case bps >= 1024*1024:
		return fmt.Sprintf("%.1fM", bps/(1024*1024))
	case bps >= 1024:
		return fmt.Sprintf("%.0fK", bps/1024)
	default:
		return fmt.Sprintf("%.0f", bps)
	}
}

func shortModel(p string) string {
	if p == "" {
		return ""
	}
	base := filepath.Base(p)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func fallbackWordmark(model string) string {
	base := shortModel(model)
	if base == "" {
		return "LLM"
	}
	parts := strings.FieldsFunc(base, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == ' '
	})
	if len(parts) == 0 {
		return "LLM"
	}
	word := parts[0]
	word = strings.TrimSuffix(word, "Team")
	word = strings.TrimSuffix(word, "AI")
	if len(word) > 6 {
		word = word[:6]
	}
	if word == "" {
		return "LLM"
	}
	return strings.ToUpper(word)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func fitString(f *braille.Font, s string, maxDots, scale int) string {
	for len(s) > 0 && f.TextWidth(s, scale) > maxDots {
		s = s[:len(s)-1]
	}
	return s
}

func (u *UI) drawChase(c *braille.Canvas, r rect) {
	phase := u.frame
	for x := r.x; x < r.x+r.w; x += 4 {
		if (x/4+phase)%4 == 0 {
			c.SetDot(x, r.y+1)
			c.SetDot(x, r.y+r.h-2)
		}
	}
	for y := r.y; y < r.y+r.h; y += 4 {
		if (y/4+phase)%4 == 0 {
			c.SetDot(r.x+1, y)
			c.SetDot(r.x+r.w-2, y)
		}
	}
}

func (u *UI) drawStarburst(c *braille.Canvas, r rect, fx, fy float64) {
	cx := r.x + int(float64(r.w)*fx)
	cy := r.y + int(float64(r.h)*fy)
	rays := 14
	length := math.Min(float64(r.w), float64(r.h)) * 0.42
	for i := 0; i < rays; i++ {
		ang := (float64(i)/float64(rays))*2*math.Pi + float64(u.frame%60)*0.02
		for d := 4.0; d < length; d += 2.0 {
			x := cx + int(math.Cos(ang)*d)
			y := cy + int(math.Sin(ang)*d)
			c.SetDot(x, y)
		}
	}
}

func (u *UI) drawProcessesASCII() {
	w, h := u.screen.Size()
	leftW := int(float64(w) * 0.30)
	rightW := int(float64(w) * 0.30)
	centerW := w - leftW - rightW
	if centerW <= 0 {
		return
	}
	x0 := leftW
	style := tcell.StyleDefault.Foreground(u.colorFor(0, 0))

	nameW := 20
	userW := 8
	if nameW+userW > centerW-19 {
		nameW = centerW - 19 - userW
		if nameW < 6 {
			nameW = 6
			userW = centerW - 19 - nameW
			if userW < 4 {
				userW = 4
			}
		}
	}
	header := fmt.Sprintf("%5s %-*s %-*s %5s %5s", "PID", nameW, "NAME", userW, "USER", "CPU", "MEM")
	u.putString(x0, 0, header, centerW, style)

	for i, p := range u.processes {
		y := i + 1
		if y >= h {
			break
		}
		line := fmt.Sprintf("%5d %-*s %-*s %5.1f %5.1f", p.PID, nameW, truncate(p.Name, nameW), userW, truncate(p.User, userW), p.CPU, p.Mem)
		u.putString(x0, y, line, centerW, style)
	}
}

func (u *UI) putString(x, y int, s string, max int, style tcell.Style) {
	i := 0
	for _, r := range s {
		if i >= max {
			break
		}
		u.screen.SetContent(x+i, y, r, nil, style)
		i++
	}
}

func (u *UI) colorFor(cx, cy int) tcell.Color {
	for _, z := range u.overrides {
		if cx >= z.x && cx < z.x+z.w && cy >= z.y && cy < z.y+z.h {
			return z.color
		}
	}
	pal := u.palette
	if len(u.activePalette) > 0 {
		pal = u.activePalette
	}
	n := len(pal)
	if n == 0 {
		return tcell.ColorWhite
	}
	t := math.Mod(float64(u.frame)*0.004+float64(cx)*0.0035+float64(cy)*0.012, 1.0)
	pos := t * float64(n)
	i := int(pos) % n
	frac := pos - math.Floor(pos)
	j := (i + 1) % n
	r := lerp(pal[i].r, pal[j].r, frac)
	g := lerp(pal[i].g, pal[j].g, frac)
	b := lerp(pal[i].b, pal[j].b, frac)
	br := u.theme.Palette.Brightness
	if br <= 0 {
		br = 1
	}
	sweep := 0.85 + 0.25*math.Sin(float64(cx)*0.12-float64(u.frame)*0.18+float64(cy)*0.04)
	br *= sweep
	if br > 1.4 {
		br = 1.4
	}
	return tcell.NewRGBColor(int32(r*br*255), int32(g*br*255), int32(b*br*255))
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func parsePalette(colors []string) []rgb {
	out := make([]rgb, 0, len(colors))
	for _, s := range colors {
		out = append(out, parseHex(s))
	}
	return out
}

func parseHex(s string) rgb {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return rgb{1, 1, 1}
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return rgb{1, 1, 1}
	}
	return rgb{
		r: float64((v>>16)&0xff) / 255,
		g: float64((v>>8)&0xff) / 255,
		b: float64(v&0xff) / 255,
	}
}
