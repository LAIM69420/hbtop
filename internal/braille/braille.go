package braille

// Canvas is a small dot-matrix canvas where one terminal cell holds a
// 2x4 Braille dot block.
type Canvas struct {
	Width  int
	Height int
	cells  []byte

	clip                       bool
	clipX0, clipY0, clipX1, clipY1 int
}

func New(width, height int) *Canvas {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return &Canvas{
		Width:  width,
		Height: height,
		cells:  make([]byte, width*height),
	}
}

func (c *Canvas) SetClip(x, y, w, h int) {
	c.clip = true
	c.clipX0 = x
	c.clipY0 = y
	c.clipX1 = x + w
	c.clipY1 = y + h
}

func (c *Canvas) ClearClip() {
	c.clip = false
}

func (c *Canvas) SetDot(x, y int) {
	if x < 0 || y < 0 || x >= c.Width*2 || y >= c.Height*4 {
		return
	}
	if c.clip && (x < c.clipX0 || x >= c.clipX1 || y < c.clipY0 || y >= c.clipY1) {
		return
	}
	cx := x / 2
	cy := y / 4
	dx := x % 2
	dy := y % 4

	var bit byte
	switch {
	case dx == 0 && dy == 0:
		bit = 0x01
	case dx == 0 && dy == 1:
		bit = 0x02
	case dx == 0 && dy == 2:
		bit = 0x04
	case dx == 1 && dy == 0:
		bit = 0x08
	case dx == 1 && dy == 1:
		bit = 0x10
	case dx == 1 && dy == 2:
		bit = 0x20
	case dx == 0 && dy == 3:
		bit = 0x40
	case dx == 1 && dy == 3:
		bit = 0x80
	}
	c.cells[cy*c.Width+cx] |= bit
}

func (c *Canvas) GetDot(x, y int) bool {
	if x < 0 || y < 0 || x >= c.Width*2 || y >= c.Height*4 {
		return false
	}
	cx := x / 2
	cy := y / 4
	dx := x % 2
	dy := y % 4
	var bit byte
	switch {
	case dx == 0 && dy == 0:
		bit = 0x01
	case dx == 0 && dy == 1:
		bit = 0x02
	case dx == 0 && dy == 2:
		bit = 0x04
	case dx == 1 && dy == 0:
		bit = 0x08
	case dx == 1 && dy == 1:
		bit = 0x10
	case dx == 1 && dy == 2:
		bit = 0x20
	case dx == 0 && dy == 3:
		bit = 0x40
	case dx == 1 && dy == 3:
		bit = 0x80
	}
	return c.cells[cy*c.Width+cx]&bit != 0
}

func (c *Canvas) FillRect(x, y, w, h int) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			c.SetDot(xx, yy)
		}
	}
}

func (c *Canvas) RenderRow(row int) []rune {
	if row < 0 || row >= c.Height {
		return []rune{}
	}
	out := make([]rune, 0, c.Width)
	for cx := 0; cx < c.Width; cx++ {
		bits := c.cells[row*c.Width+cx]
		if bits == 0 {
			out = append(out, ' ')
		} else {
			out = append(out, rune(0x2800+int(bits)))
		}
	}
	return out
}

func (c *Canvas) Render() [][]rune {
	out := make([][]rune, c.Height)
	for r := 0; r < c.Height; r++ {
		out[r] = c.RenderRow(r)
	}
	return out
}
