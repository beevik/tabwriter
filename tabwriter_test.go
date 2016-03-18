package tabwriter

import (
	"fmt"
	"os"
	"testing"

	tw "text/tabwriter"
)

func TestTabWriter(t *testing.T) {
	// w1 := NewWriter(os.Stdout, 0, 8, 1, ' ', 0)
	// fmt.Fprintln(w1, "lego [global options] command [command options] [arguments...]")
	// fmt.Fprintln(w1, " ")
	// fmt.Fprintln(w1, "one\ttwo")
	// w1.Flush()

	// fmt.Printf("\n\n---\n\n")

	// w2 := tw.NewWriter(os.Stdout, 0, 8, 1, ' ', 0)
	// fmt.Fprintln(w2, "lego [global options] command [command options] [arguments...]")
	// fmt.Fprintln(w2, " ")
	// fmt.Fprintln(w2, "one\ttwo")
	// w2.Flush()

	w1 := NewWriter(os.Stdout, 0, 8, 1, '_', 0)
	fmt.Fprintln(w1, "a\tb\tc\td\t.")
	fmt.Fprint(w1, "1234\t1234")
	fmt.Fprint(w1, "56789\t123456")
	fmt.Fprintf(w1, "7\t123456789\t1234567890123456789012345678901234567890\t.\n")
	fmt.Fprintln(w1, "AAA\tB\tC\t")
	fmt.Fprintln(w1, "123\t12345\t1234567\t123456\t200\t18")
	w1.Flush()

	fmt.Println("\n---\n")

	w2 := tw.NewWriter(os.Stdout, 0, 8, 1, '_', 0)
	fmt.Fprintln(w2, "a\tb\tc\td\t.")
	fmt.Fprint(w2, "1234\t1234")
	fmt.Fprint(w2, "56789\t123456")
	fmt.Fprintf(w2, "7\t123456789\t1234567890123456789012345678901234567890\t.\n")
	fmt.Fprintln(w2, "AAA\tB\tC\t")
	fmt.Fprintln(w2, "123\t12345\t1234567\t123456\t200\t18")
	w2.Flush()
}
