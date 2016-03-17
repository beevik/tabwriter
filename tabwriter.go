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

type cell struct {
	size     int // Number of bytes in cell
	width    int // Number of runes in the cell
	maxwidth int // Maximum width seen in this cell's column so far
}

type line struct {
	cells []cell
}

type Writer struct {
	output   io.Writer
	minwidth int
	tabwidth int
	padding  int
	padchar  byte
	flags    uint
	colflags []uint

	buf   bytes.Buffer // unformatted bytes accumulated until flush
	lines []line       // lines accumulated until flush
	cell  cell         // currently updating cell
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
	}
	w.reset()
	return w
}

func (w *Writer) reset() {
	w.buf.Reset()
	w.cell = cell{}
	w.lines = nil
	w.addLine()
}

func (w *Writer) addTextToCell(text []byte) {
	w.buf.Write(text)
	w.cell.size += len(text)
}

func (w *Writer) addCellToLine() {
	// Calculate the cell's width (the number of runes).
	b := w.buf.Bytes()
	w.cell.width = utf8.RuneCount(b[len(b)-w.cell.size:])

	linecount := len(w.lines)
	curr := &w.lines[linecount-1]

	w.cell.maxwidth = w.cell.width
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

func (w *Writer) addLine() {
	w.lines = append(w.lines, line{[]cell{}})
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

	// Propagate the accumulated maxwidths from the bottom lines upward.
	for i := len(w.lines) - 1; i > 0; i-- {
		curr := &w.lines[i]
		prev := &w.lines[i-1]
		for j, jc := 0, min(len(prev.cells), len(curr.cells)); j < jc; j++ {
			prev.cells[j].maxwidth =
				max(prev.cells[j].maxwidth, curr.cells[j].maxwidth)
		}
	}

	// Format the lines.
	p := 0
	for _, l := range w.lines {
		for j, c := range l.cells {
			ralign := (w.getflags(j) & AlignRight) != 0
			if ralign {
				w.output.Write(bytes.Repeat([]byte{' '}, c.maxwidth-c.width))
				w.output.Write(w.buf.Bytes()[p : p+c.size])
				w.output.Write([]byte{' '})
			} else {
				w.output.Write(w.buf.Bytes()[p : p+c.size])
				w.output.Write(bytes.Repeat([]byte{' '}, c.maxwidth-c.width+1))
			}
			p = p + c.size
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

func (w *Writer) getflags(col int) uint {
	flags := w.flags
	if col < len(w.colflags) && (w.colflags[col]&specified) != 0 {
		flags = w.colflags[col]
	}
	return flags
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
