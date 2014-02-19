package bg

import (
	"bytes"
	"testing"
)

func TestRle(t *testing.T) {
	in := []uint8{0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 0, 0, 0, 0, 0, 4, 5, 6, 0, 0}
	expected := []byte{0, 7, 1, 2, 3, 0, 4, 4, 5, 6, 0, 1}

	out := rleBam(in, 0)
	if bytes.Compare(out, expected) != 0 {
		t.Errorf("rleBam %q != %q", expected, out)
	}

}
