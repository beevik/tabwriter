package tabwriter

import (
	"fmt"
	"os"
	"testing"
	tw "text/tabwriter"
)

func TestTabWriter(t *testing.T) {
	w1 := NewWriter(os.Stdout, 0, 8, 1, ' ', 0)
	w1.SetColumnFlags(2, AlignRight)
	w1.SetColumnFlags(4, AlignRight)
	fmt.Fprintln(w1, "a\tb\tc\td\t.")
	fmt.Fprint(w1, "1234\t1234")
	fmt.Fprint(w1, "56789\t123456")
	fmt.Fprintln(w1, "7\t123456789\t1234567890123456789012345678901234567890\t.")
	fmt.Fprintln(w1, "AAA\tB\tC\t\t.\t.")
	fmt.Fprintln(w1, "123\t12345\t1234567\t123456\t200\t18")
	w1.Flush()

	fmt.Println("\n---\n")

	w2 := tw.NewWriter(os.Stdout, 0, 8, 1, '_', tw.AlignRight)
	fmt.Fprintln(w2, "a\tb\tc\td\t.")
	fmt.Fprint(w2, "1234\t1234")
	fmt.Fprint(w2, "56789\t123456")
	fmt.Fprintln(w2, "7\t123456789\t1234567890123456789012345678901234567890\t.")
	fmt.Fprintln(w2, "AAA\tB\tC\t\t.\t.")
	fmt.Fprintln(w2, "123\t12345\t1234567\t123456\t200\t18")
	w2.Flush()
}
