// Package tabwriter implements a write filter (tabwriter.Writer) that
// translates tabbed columns in input into properly aligned text.
//
// It is a drop-in replacement for the standard library's text/tabwriter
// package.
//
// Differences between this tabwriter and the go standard library's version:
//
// This tabwriter allows setting formatting flags per-column. So you can have
// some columns that are align-right and some that are align-left.
//
// This tabwriter properly right-aligns the last column. The standard
// library's version always left-aligns the last column regardless of the
// formatting flags.
//
// This tabwriter processes '\r' as a "new-row" indicator, meaning that
// all text between it and the next '\n' appears indented on a new line
// without affecting subsequent tab formatting. This is most useful for
// usage text descriptions.
//
// This tabwriter always outputs a newline after a flush.
//
// This library does not support HTML filtering, escaped text sequences,
// tab-indenting for padchar's other than '\t', or discarding of empty
// columns.
//
// This library ignores '\v' and '\f' characters.
package tabwriter

import (
	"bytes"
	"io"
	"unicode/utf8"
)

const (
	// Force right-alignment of a column's content.
	AlignRight uint = 1 << iota

	specified
)

// A Writer is a filter that inserts padding around tab-delimited columns in
// its input to align them in the output.
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
	w.lines = w.lines[0:0]
	w.addNewLine()
}

// getFlags returns the tabwriter flags that should be used for column col.
func (w *Writer) getFlags(col int) uint {
	if col < len(w.colflags) && (w.colflags[col]&specified) != 0 {
		return w.colflags[col]
	} else {
		return w.flags
	}
}

// addTextToCell updates the contents of the working cell.
func (w *Writer) addTextToCell(text []byte) {
	w.buf.Write(text)
	w.cell.size += len(text)
}

// addCellToLine adds the current working cell to the current working line and
// starts a new working cell.
func (w *Writer) addCellToLine(term bool) {
	// Calculate the cell's width (the number of runes).
	b := w.buf.Bytes()
	w.cell.width = utf8.RuneCount(b[len(b)-w.cell.size:])

	linecount := len(w.lines)
	line := &w.lines[linecount-1]

	// If a line is being terminated and the working cell is empty, we're
	// done.
	if term && w.cell.size == 0 {
		w.cell = cell{}
		if len(line.cells) == 0 {
			// Flush on an empty line.
			w.Flush()
		}
		return
	}

	w.cell.maxwidth = max(w.minwidth, w.cell.width+w.padding)
	if linecount > 1 {
		// Examine the cell in the previous line at the same column. Compute
		// this cell's maxwidth based on that cell's maxwidth and this cell's
		// width. This causes the maxwidth for each column to accumulate
		// downwards. When we flush, we'll traverse the lines in reverse and
		// copy the per-column maxwidth values upwards.
		col := len(line.cells)
		prev := &w.lines[linecount-2]
		if col < len(prev.cells) {
			w.cell.maxwidth = max(w.cell.maxwidth, prev.cells[col].maxwidth)
		}
	}

	line.cells = append(line.cells, w.cell)
	w.cell = cell{}
}

// addNewLine adds a new, empty line to the working set.
func (w *Writer) addNewLine() {
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
func (w *Writer) writeCell(text []byte, padding int, flags uint, term bool) {
	switch {
	case (term && (flags&AlignRight == 0)) || padding == 0:
		// Don't pad the last cell in a left-aligned line.
		w.output.Write(text)

	case w.padchar == '\t':
		// Write text and then pad with tabs. Never right-align when padding
		// with tabs.
		w.output.Write(text)
		w.writePadding((padding + w.tabwidth - 1) / w.tabwidth)

	case (flags & AlignRight) != 0:
		// When aligning right, use one of the pad characters on the right
		// side of the text. This way, two adjacent columns that are align-
		// right and align-left will not touch one another.
		w.writePadding(padding - 1)
		w.output.Write(text)
		if !term {
			w.writePadding(1)
		}

	default:
		// When aligning left, pad on the right.
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

// NewWriter creates and initializes a new tabwriter.Writer.
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

// Init initializes a tabwriter.Writer, which filters its output to the
// writer in the first parameter. The remaining parameters control formatting:
//
//  minwidth    minimal cell width including padding
//  tabwidth    width of a tab in spaces, used to determine location of
//              tab stops
//  padding     extra pad characters added to cells
//  padchar     the character to use for padding. If tab ('\t') is used, then
//              all padding is tabbed and left-aligned using tabwidth.
//  flags       formatting control flags
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

// Write writes buf to the writer w, returning the number of bytes written
// and any errors encountered while writing to the underlying stream.
func (w *Writer) Write(buf []byte) (n int, err error) {
	n = 0
	for i, ch := range buf {
		switch ch {
		case '\n':
			w.addTextToCell(buf[n:i])
			w.addCellToLine(true)
			n = i + 1
			w.addNewLine()

		case '\t':
			w.addTextToCell(buf[n:i])
			w.addCellToLine(false)
			n = i + 1
		}
	}

	w.addTextToCell(buf[n:])
	n = len(buf)
	return
}

// Flush triggers the formatting and output of tabbed text to the underlying
// stream.
func (w *Writer) Flush() {
	if w.cell.size > 0 {
		w.addCellToLine(true)
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
		for col, c := range l.cells {
			text := w.buf.Bytes()[p : p+c.size]
			padding := c.maxwidth - c.width
			term := col+1 == len(l.cells)
			w.writeCell(text, padding, w.getFlags(col), term)
			p += c.size
		}
		w.output.Write([]byte{'\n'})
	}

	w.reset()
}

// SetColumnFlags sets column-specific formatting flags for column 'col'.
func (w *Writer) SetColumnFlags(col int, flags uint) {
	if col >= len(w.colflags) {
		colflags := make([]uint, col+1)
		copy(colflags, w.colflags)
		w.colflags = colflags
	}
	w.colflags[col] = flags | specified
}
