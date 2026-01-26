package models

import (
	"testing"
	"time"
)

func TestProvenance_IsApproved(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		provenance Provenance
		want       bool
	}{
		{
			name: "learned and approved",
			provenance: Provenance{
				SourceType: SourceTypeLearned,
				ApprovedBy: "alice",
				ApprovedAt: &now,
			},
			want: true,
		},
		{
			name: "learned but not approved",
			provenance: Provenance{
				SourceType: SourceTypeLearned,
				ApprovedBy: "",
			},
			want: false,
		},
		{
			name: "authored - not applicable",
			provenance: Provenance{
				SourceType: SourceTypeAuthored,
				Author:     "bob",
			},
			want: false,
		},
		{
			name: "imported - not applicable",
			provenance: Provenance{
				SourceType: SourceTypeImported,
				Package:    "github.com/example/behaviors",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.provenance.IsApproved()
			if got != tt.want {
				t.Errorf("IsApproved() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvenance_IsPending(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		provenance Provenance
		want       bool
	}{
		{
			name: "learned and pending (no approver)",
			provenance: Provenance{
				SourceType: SourceTypeLearned,
				ApprovedBy: "",
			},
			want: true,
		},
		{
			name: "learned and approved",
			provenance: Provenance{
				SourceType: SourceTypeLearned,
				ApprovedBy: "alice",
				ApprovedAt: &now,
			},
			want: false,
		},
		{
			name: "authored - not pending",
			provenance: Provenance{
				SourceType: SourceTypeAuthored,
				Author:     "bob",
			},
			want: false,
		},
		{
			name: "imported - not pending",
			provenance: Provenance{
				SourceType: SourceTypeImported,
				Package:    "github.com/example/behaviors",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.provenance.IsPending()
			if got != tt.want {
				t.Errorf("IsPending() = %v, want %v", got, tt.want)
			}
		})
	}
}
