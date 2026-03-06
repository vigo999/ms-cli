package ui

import "testing"

func TestMaskSensitiveInput(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "model key masked", in: "/model key secret", want: "/model key ****"},
		{name: "model key with spaces masked", in: "  /model   key   secret  ", want: "/model key ****"},
		{name: "non key command unchanged", in: "/model gpt-4o", want: "/model gpt-4o"},
		{name: "normal input unchanged", in: "hello", want: "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSensitiveInput(tt.in)
			if got != tt.want {
				t.Fatalf("maskSensitiveInput(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
