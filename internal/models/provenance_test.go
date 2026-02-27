package models

import "testing"

func TestExtractPackageVersion(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		want     string
	}{
		{
			"valid provenance",
			map[string]interface{}{
				"provenance": map[string]interface{}{
					"package_version": "1.0.0",
					"package":         "floop-core",
				},
			},
			"1.0.0",
		},
		{
			"nil metadata",
			nil,
			"",
		},
		{
			"no provenance key",
			map[string]interface{}{
				"other": "value",
			},
			"",
		},
		{
			"provenance wrong type",
			map[string]interface{}{
				"provenance": "not-a-map",
			},
			"",
		},
		{
			"no package_version",
			map[string]interface{}{
				"provenance": map[string]interface{}{
					"package": "floop-core",
				},
			},
			"",
		},
		{
			"empty metadata",
			map[string]interface{}{},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPackageVersion(tt.metadata)
			if got != tt.want {
				t.Errorf("ExtractPackageVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractPackageName(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		want     string
	}{
		{
			"valid provenance",
			map[string]interface{}{
				"provenance": map[string]interface{}{
					"package":         "floop-core",
					"package_version": "1.0.0",
				},
			},
			"floop-core",
		},
		{
			"nil metadata",
			nil,
			"",
		},
		{
			"no provenance key",
			map[string]interface{}{
				"other": "value",
			},
			"",
		},
		{
			"provenance wrong type",
			map[string]interface{}{
				"provenance": "not-a-map",
			},
			"",
		},
		{
			"no package key",
			map[string]interface{}{
				"provenance": map[string]interface{}{
					"package_version": "1.0.0",
				},
			},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPackageName(tt.metadata)
			if got != tt.want {
				t.Errorf("ExtractPackageName() = %q, want %q", got, tt.want)
			}
		})
	}
}
