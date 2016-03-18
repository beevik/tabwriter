package tabwriter

import (
	"fmt"
	"os"
	"testing"
	tw "text/tabwriter"
)

func TestTabWriter(t *testing.T) {
	w := NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	// w.SetColumnFlags(4, AlignRight)

	fmt.Fprintln(w, "a\tb\tc\td\t.")
	fmt.Fprintln(w, "1234\t123456789\t1234567\t123456789\t1234567890123456789012345678901234567890\t.")
	fmt.Fprintln(w, "AAA\tB\tC\t\t.\t.")
	fmt.Fprint(w, "123\t12345\t1234567\t123456\t200\t18")
	w.Flush()

	fmt.Println("\n\n")

	w2 := tw.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)

	fmt.Fprintln(w2, "a\tb\tc\td\t.")
	fmt.Fprint(w2, "123\t1234")
	fmt.Fprint(w2, "56789\t123456")
	fmt.Fprintln(w2, "7\t123456789\t1234567890123456789012345678901234567890\t.")
	fmt.Fprintln(w2, "AAA\tB\tC\t\t.\t.")
	fmt.Fprint(w2, "123\t12345\t1234567\t123456\t200\t18\n")
	w2.Flush()
}
