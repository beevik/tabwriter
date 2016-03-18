// Package tabwriter implements a write filter (tabwriter.Writer) that
// translates tabbed columns in input into properly aligned text. It is a
// drop-in replacement for the standard library's text/tabwriter package.
//
// There are several differences between this tabwriter and the go standard
// library's tabwriter.
//
// This tabwriter allows per-column format settings. For instance, you can
// have some columns that are align-right and some that are align-left.
//
// This tabwriter processes '\r' as a "description row" indicator. All text
// between '\r' and the next '\n' is treated as a specially indented and
// word-wrapped description block. For example, the string
// "--flags\rFormatting flags\n" results in the following output:
//
//   --flags
//           Formatting flags
//
// Each '\r' that appears before a '\n' is output as another word-wrapped
// newline/indent combo.
//
// This tabwriter always outputs a newline after a flush.
//
// This library does not support HTML filtering, escaped text sequences,
// tab-indenting for padchar's other than '\t', or discarding of empty
// columns.
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
	output            io.Writer  // underlying output stream
	tabwidth          int        // spaces between tab stops
	padchar           byte       // character to use for cell padding
	format            format     // default format
	formatColumn      []format   // per-column format
	formatColumnBits  uint64     // bit mask of valid formatColumn entries
	formatDescription formatDesc // format settings for description rows

	padbytes []byte       // array of padchars to use when padding
	buf      bytes.Buffer // unformatted bytes accumulated until flush
	lines    []line       // lines accumulated until flush
	cell     cell         // current working cell

	addCell  func(w *Writer, term bool)
	descmode bool // currently in description update mode
}

// format describes the settings to use for cell text output.
type format struct {
	minwidth int  // minimum width of cell including padding
	padding  int  // number of extra padding chars in a cell
	flags    uint // format flags
}

// formatDesc describes the settings to use for description text output.
type formatDesc struct {
	indent   int // Columns to indent descriptions
	wordwrap int // Column at which to word-wrap descriptions
}

type cell struct {
	size     int  // number of bytes in cell
	width    int  // number of runes in the cell
	maxwidth int  // maximum width seen in this cell's column so far
	term     bool // last cell in line
}

type line struct {
	cells       []cell // All non-description cells in the row
	description cell   // The description cell (if any)
}

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

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
func (w *Writer) getFormat(col int) format {
	if col >= 64 || w.formatColumnBits&(uint64(1)<<uint(col)) == 0 {
		return w.format
	} else {
		return w.formatColumn[col]
	}
}

// addTextToCell updates the contents of the working cell.
func (w *Writer) addTextToCell(text []byte) {
	w.buf.Write(text)
	w.cell.size += len(text)
}

// addCellToLine finalizes the working cell and appends it to the working
// line.
func (w *Writer) addCellToLine(term bool) {
	// Calculate the cell's width (the number of runes).
	b := w.buf.Bytes()
	w.cell.width = utf8.RuneCount(b[len(b)-w.cell.size:])

	linecount := len(w.lines)
	line := &w.lines[linecount-1]

	// Special case: the current working cell is empty and it terminates the
	// line.
	if term && (w.cell.size == 0) {
		// If the current line is empty, flush. Otherwise, mark the previous
		// cell in the line as the terminator.
		if len(line.cells) == 0 {
			w.Flush()
		} else {
			line.cells[len(line.cells)-1].term = true
		}

		w.cell = cell{}
		return
	}

	w.cell.term = term

	col := len(line.cells)
	format := w.getFormat(col)

	w.cell.maxwidth = max(format.minwidth, w.cell.width+format.padding)
	if linecount > 1 {
		// Examine the cell in the previous line at the same column. Compute
		// this cell's maxwidth based on that cell's maxwidth and this cell's
		// width. This causes the maxwidth for each column to accumulate
		// downwards. When we flush, we'll traverse the lines in reverse and
		// copy the per-column maxwidth values upwards.
		prev := &w.lines[linecount-2]
		if col < len(prev.cells)-1 {
			w.cell.maxwidth = max(w.cell.maxwidth, prev.cells[col].maxwidth)
		}
	}

	line.cells = append(line.cells, w.cell)
	w.cell = cell{}
}

// addCellToLine finalizes the working cell and sets the working line's
// description to it.
func (w *Writer) addCellToDescription(term bool) {
	b := w.buf.Bytes()
	w.cell.width = utf8.RuneCount(b[len(b)-w.cell.size:])
	w.lines[len(w.lines)-1].description = w.cell
	w.cell = cell{}
}

// addNewLine adds a new, empty line to the working set.
func (w *Writer) addNewLine() {
	w.lines = append(w.lines, line{[]cell{}, cell{}})
	w.addCell = (*Writer).addCellToLine
	w.descmode = false
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
func (w *Writer) writeCell(text []byte, padding int, format format, term bool) {
	switch {
	case padding == 0:
		fallthrough
	case term && (format.flags&AlignRight == 0):
		// Don't pad the terminating cell in a left-aligned line.
		w.output.Write(text)

	case w.padchar == '\t':
		// Write text and then pad with tabs. Never right-align when padding
		// with tabs.
		w.output.Write(text)
		w.writePadding((padding + w.tabwidth - 1) / w.tabwidth)

	case (format.flags & AlignRight) != 0:
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

func (w *Writer) writeDescription(text []byte) {
	for p := 0; p < len(text); {

		// Output indent.
		col := w.formatDescription.indent
		if w.padchar == '\t' {
			w.writePadding((col + w.tabwidth - 1) / w.tabwidth)
		} else {
			w.writePadding(col)
		}

		// Scan until '\r' or end of text. Break overly long lines at the last
		// possible space.
		p0, lastspace := p, -1
		for {
			if p >= len(text) || text[p] == '\r' {
				w.output.Write(text[p0:p])
				w.output.Write(newline)
				p++
				break
			}

			if text[p] == ' ' {
				lastspace = p
			}

			_, size := utf8.DecodeRune(text[p:])
			p += size
			col++

			if col > w.formatDescription.wordwrap && lastspace != -1 {
				w.output.Write(text[p0:lastspace])
				w.output.Write(newline)
				p = lastspace + 1
				break
			}
		}
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
		output:            output,
		tabwidth:          tabwidth,
		padchar:           padchar,
		format:            format{minwidth, padding, flags},
		formatColumn:      []format{},
		formatDescription: formatDesc{8, 72},
		padbytes:          bytes.Repeat([]byte{padchar}, 8),
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
	w.tabwidth = tabwidth
	w.padchar = padchar
	w.format = format{minwidth, padding, flags}
	w.formatColumn = []format{}
	w.formatDescription = formatDesc{8, 72}
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
		case '\t', '\v':
			w.addTextToCell(buf[n:i])
			if w.descmode {
				// If the current line is in description mode, replace
				// tabs with spaces.
				w.addTextToCell(space)
			} else {
				w.addCell(w, false)
			}
			n = i + 1

		case '\n':
			w.addTextToCell(buf[n:i])
			w.addCell(w, true)
			n = i + 1
			w.addNewLine()

		case '\f':
			w.addTextToCell(buf[n:i])
			w.addCell(w, true)
			n = i + 1
			w.Flush()

		case '\r':
			if !w.descmode {
				w.addTextToCell(buf[n:i])
				w.addCell(w, true)
				n = i + 1
				w.descmode = true
				w.addCell = (*Writer).addCellToDescription
			}
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
		w.addCell(w, true)
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
		for j, jc := 0, min(len(prev.cells)-1, len(curr.cells)-1); j < jc; j++ {
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
			w.writeCell(text, padding, w.getFormat(j), c.term)
			p += c.size
		}
		w.output.Write(newline)
		if l.description.size > 0 {
			text := w.buf.Bytes()[p : p+l.description.size]
			w.writeDescription(text)
			p += l.description.size
		}
	}

	w.reset()
}

// SetColumnFlags sets column-specific format settings for column 'col'.
func (w *Writer) SetColumnFormat(col int, minwidth int, padding int, flags uint) {
	if col >= 64 {
		// Disallow custom formatting on more than 64 columns.
		return
	}
	if col >= len(w.formatColumn) {
		c := make([]format, col+1)
		copy(c, w.formatColumn)
		w.formatColumn = c
	}
	w.formatColumn[col] = format{minwidth, padding, flags}
	w.formatColumnBits |= uint64(1) << uint(col)
}

// SetDescriptionFormat sets format settings for description output.
func (w *Writer) SetDescriptionFormat(indent, wordwrap int) {
	w.formatDescription = formatDesc{indent, wordwrap}
}
