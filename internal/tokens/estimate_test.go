package tokens

import "testing"

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty string", "", 0},
		{"single char", "a", 1},
		{"four chars (1 token)", "abcd", 1},
		{"five chars (2 tokens)", "abcde", 2},
		{"eight chars (2 tokens)", "abcdefgh", 2},
		{"typical short text", "hello world", 3},
		{"longer text", "This is a longer piece of text that should estimate to more tokens", 17},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.input)
			if got != tt.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
