package pack

import (
	"testing"
)

func TestValidatePackID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid simple", "floop/core", false},
		{"valid with hyphens", "my-org/my-pack", false},
		{"valid with numbers", "org123/pack456", false},
		{"valid all hyphens and nums", "a-b-c/x-1-2", false},
		{"invalid no slash", "noslash", true},
		{"invalid uppercase", "UPPER/case", true},
		{"invalid mixed case", "Foo/bar", true},
		{"invalid space", "space /bar", true},
		{"invalid trailing slash", "foo/", true},
		{"invalid leading slash", "/bar", true},
		{"invalid double slash", "foo/bar/baz", true},
		{"invalid empty", "", true},
		{"invalid underscore", "foo_bar/baz", true},
		{"invalid dot", "foo.bar/baz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePackID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePackID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}
