package anim

import (
	"encoding/json"
	"os"

	"hbtop/internal/braille"
)

type Project struct {
	Magic  string     `json:"magic"`
	CellW  int        `json:"cell_w"`
	CellH  int        `json:"cell_h"`
	FPS    int        `json:"fps"`
	Frames [][]string `json:"frames"`
}

func Load(path string) (*Project, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	if p.CellW <= 0 {
		p.CellW = 20
	}
	if p.CellH <= 0 {
		p.CellH = 8
	}
	if p.FPS <= 0 {
		p.FPS = 12
	}
	return &p, nil
}

func (p *Project) Len() int { return len(p.Frames) }

func (p *Project) Frame(i int) []string {
	if len(p.Frames) == 0 {
		return nil
	}
	return p.Frames[i%len(p.Frames)]
}

// Draw renders a frame at dot offset (x, y).
func (p *Project) Draw(c *braille.Canvas, frame, x, y int) {
	rows := p.Frame(frame)
	for cy, row := range rows {
		for cx, r := range row {
			bits := brailleBits(r)
			if bits == 0 {
				continue
			}
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					if bits&bitFor(dx, dy) != 0 {
						c.SetDot(x+cx*2+dx, y+cy*4+dy)
					}
				}
			}
		}
	}
}

func brailleBits(r rune) byte {
	if r >= 0x2800 && r <= 0x28FF {
		return byte(r - 0x2800)
	}
	return 0
}

func bitFor(dx, dy int) byte {
	switch {
	case dx == 0 && dy == 0:
		return 0x01
	case dx == 0 && dy == 1:
		return 0x02
	case dx == 0 && dy == 2:
		return 0x04
	case dx == 1 && dy == 0:
		return 0x08
	case dx == 1 && dy == 1:
		return 0x10
	case dx == 1 && dy == 2:
		return 0x20
	case dx == 0 && dy == 3:
		return 0x40
	case dx == 1 && dy == 3:
		return 0x80
	}
	return 0
}
