package measure

import "testing"

func TestTruncateBody(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		n    int
		want string
	}{
		{"empty", nil, 10, ""},
		{"short", []byte("hello"), 10, "hello"},
		{"exact", []byte("hello"), 5, "hello"},
		{"over", []byte("hello world"), 5, "hello..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateBody(tt.body, tt.n)
			if got != tt.want {
				t.Errorf("truncateBody(%q, %d) = %q, want %q", tt.body, tt.n, got, tt.want)
			}
		})
	}
}
