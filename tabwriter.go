package tabwriter

import (
	"bytes"
	"io"
	"unicode/utf8"
)

const (
	AlignRight uint = 1 << iota
	specified
)

type Writer struct {
	output   io.Writer
	minwidth int
	tabwidth int
	padding  int
	padchar  byte
	flags    uint
	colflags []uint

	padbytes []byte       // array of padchars to use when padding
	buf      bytes.Buffer // unformatted bytes accumulated until flush
	lines    []line       // lines accumulated until flush
	cell     cell         // currently updating cell
}

type cell struct {
	size     int // number of bytes in cell
	width    int // number of runes in the cell
	maxwidth int // maximum width seen in this cell's column so far
}

type line struct {
	cells []cell
}

func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func max(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

// reset completely resets the state of the tabwriter.
func (w *Writer) reset() {
	w.buf.Reset()
	w.cell = cell{}
	w.lines = nil
	w.addLine()
}

// getFlags returns the tabwriter flags that should be used for column col.
func (w *Writer) getFlags(col int) uint {
	flags := w.flags
	if col < len(w.colflags) && (w.colflags[col]&specified) != 0 {
		flags = w.colflags[col]
	}
	return flags
}

// addTextToCell updates the contents of the working cell.
func (w *Writer) addTextToCell(text []byte) {
	w.buf.Write(text)
	w.cell.size += len(text)
}

// addCellToLine adds the current working cell to the current working line
// and starts a new working cell.
func (w *Writer) addCellToLine() {
	// Calculate the cell's width (the number of runes).
	b := w.buf.Bytes()
	w.cell.width = utf8.RuneCount(b[len(b)-w.cell.size:])

	linecount := len(w.lines)
	curr := &w.lines[linecount-1]

	w.cell.maxwidth = max(w.minwidth, w.cell.width+w.padding)
	if linecount > 1 {
		// Examine the cell in the previous line at the same column. Compute
		// this cell's maxwidth based on that cell's maxwidth and this cell's
		// width. This causes the maxwidth for each column to accumulate
		// downwards. When we flush, we'll traverse the lines in reverse and
		// copy the per-column maxwidth values upwards.
		col := len(curr.cells)
		prev := &w.lines[linecount-2]
		if col < len(prev.cells) {
			w.cell.maxwidth = max(w.cell.maxwidth, prev.cells[col].maxwidth)
		}
	}

	curr.cells = append(curr.cells, w.cell)
	w.cell = cell{}
}

// addLine adds a new, empty line to the working set.
func (w *Writer) addLine() {
	w.lines = append(w.lines, line{[]cell{}})
}

// tabifyLine adjusts the maxwidth of each cell in a line so that each
// cell begins on a tab stop.
func (w *Writer) tabifyLine(line *line) {
	for i := range line.cells {
		c := &line.cells[i]
		remainder := c.maxwidth % w.tabwidth
		if remainder != 0 {
			c.maxwidth += w.tabwidth - remainder
		}
	}
}

// writeCell outputs a cell's contents and its padding.
func (w *Writer) writeCell(text []byte, padding int, flags uint) {
	if w.padchar == '\t' {
		w.output.Write(text)
		stops := (padding + w.tabwidth - 1) / w.tabwidth
		w.writePadding(stops)
	} else if (flags & AlignRight) != 0 {
		w.writePadding(padding)
		w.output.Write(text)
	} else {
		w.output.Write(text)
		w.writePadding(padding)
	}
}

// writePadding outputs n pad characters.
func (w *Writer) writePadding(n int) {
	for n > len(w.padbytes) {
		w.output.Write(w.padbytes)
		n -= len(w.padbytes)
	}
	w.output.Write(w.padbytes[:n])
}

func NewWriter(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *Writer {
	w := &Writer{
		output:   output,
		minwidth: minwidth,
		tabwidth: tabwidth,
		padding:  padding,
		padchar:  padchar,
		flags:    flags,
		colflags: []uint{},
		padbytes: bytes.Repeat([]byte{padchar}, 8),
	}
	w.reset()
	return w
}

func (w *Writer) Init(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *Writer {
	w.output = output
	w.minwidth = minwidth
	w.tabwidth = tabwidth
	w.padding = padding
	w.padchar = padchar
	w.flags = flags
	w.colflags = []uint{}
	w.padbytes = bytes.Repeat([]byte{padchar}, 8)
	w.reset()
	return w
}

func (w *Writer) Write(buf []byte) (n int, err error) {
	n = 0
	for i, ch := range buf {
		switch ch {
		case '\n':
			w.addTextToCell(buf[n:i])
			w.addCellToLine()
			n = i + 1
			w.addLine()

		case '\t':
			w.addTextToCell(buf[n:i])
			w.addCellToLine()
			n = i + 1
		}
	}

	w.addTextToCell(buf[n:])
	n = len(buf)
	return
}

func (w *Writer) Flush() {
	if w.cell.size > 0 {
		w.addCellToLine()
	}

	// If the last line is empty, strip it.
	if len(w.lines[len(w.lines)-1].cells) == 0 {
		w.lines = w.lines[:len(w.lines)-1]
	}

	// Adjust each line's cell maxwidth values
	for i := len(w.lines) - 1; i > 0; i-- {
		curr := &w.lines[i]

		if w.padchar == '\t' {
			// Adjust column widths to hit tab stops.
			w.tabifyLine(curr)
		}

		// Propagate the accumulated maxwidths from the bottom lines upward.
		prev := &w.lines[i-1]
		for j, jc := 0, min(len(prev.cells), len(curr.cells)); j < jc; j++ {
			prev.cells[j].maxwidth =
				max(prev.cells[j].maxwidth, curr.cells[j].maxwidth)
		}
	}

	// Format and output the lines.
	p := 0
	for _, l := range w.lines {
		for j, c := range l.cells {
			text := w.buf.Bytes()[p : p+c.size]
			padding := c.maxwidth - c.width
			w.writeCell(text, padding, w.getFlags(j))
			p += c.size
		}
		w.output.Write([]byte{'\n'})
	}

	w.reset()
}

func (w *Writer) SetColumnFlags(col int, flags uint) {
	if col >= len(w.colflags) {
		colflags := make([]uint, col+1)
		copy(colflags, w.colflags)
		w.colflags = colflags
	}
	w.colflags[col] = flags | specified
}
