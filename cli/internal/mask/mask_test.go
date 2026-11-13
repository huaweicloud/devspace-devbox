package mask

import "testing"

func TestKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"short", "*****"},
		{"12345678", "********"},
		{"123456789", "1234*6789"},
		{"sk-abcdefghijklmnop", "sk-a***********mnop"},
	}
	for _, c := range cases {
		if got := Key(c.in); got != c.want {
			t.Errorf("Key(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
