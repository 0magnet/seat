package seat

import "testing"

func TestCtrlAltDigitIsASwitch(t *testing.T) {
	cases := []struct {
		ctrl, alt, shift, meta bool
		code, key              string
		want                   int
	}{
		{true, true, false, false, "Digit1", "1", 0},
		{true, true, false, false, "Digit9", "9", 8},
		// AltGr layouts: Ctrl+Alt+7 types a brace, which belongs to the program.
		{true, true, false, false, "Digit7", "{", -1},
		{true, true, false, false, "Digit0", "0", -1},
		{true, false, false, false, "Digit1", "1", -1},
		{false, true, false, false, "Digit1", "1", -1},
		{true, true, true, false, "Digit1", "!", -1},
		{true, true, false, true, "Digit1", "1", -1},
		{true, true, false, false, "Numpad1", "1", -1},
		{true, true, false, false, "KeyA", "a", -1},
	}
	for _, c := range cases {
		if got := keyIndex(c.ctrl, c.alt, c.shift, c.meta, c.code, c.key); got != c.want {
			t.Errorf("ctrl=%v alt=%v shift=%v meta=%v %s %q: %d, want %d", c.ctrl, c.alt, c.shift, c.meta, c.code, c.key, got, c.want)
		}
	}
}
