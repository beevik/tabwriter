package tabwriter

import (
	"fmt"
	"os"
	"testing"
)

func TestTabWriter(t *testing.T) {
	w := NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	w.SetColumnFlags(4, AlignRight)

	fmt.Fprintln(w, "a\tb\tc\td\t.")
	fmt.Fprint(w, "123\t1234")
	fmt.Fprint(w, "56789\t123456")
	fmt.Fprintln(w, "7\t123456789\t1234567890123456789012345678901234567890\t.")
	fmt.Fprintln(w, "AAAA\tB\tC\t\t.\t.")
	fmt.Fprint(w, "123\t12345\t1234567\t123456\t200\t18")
	w.Flush()
}
