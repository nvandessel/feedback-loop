package tagging

import (
	"reflect"
	"testing"
)

func TestMergeTags(t *testing.T) {
	dict := NewDictionary()

	tests := []struct {
		name     string
		inferred []string
		extra    []string
		dict     *Dictionary
		want     []string
	}{
		{
			name:     "both nil",
			inferred: nil,
			extra:    nil,
			dict:     dict,
			want:     nil,
		},
		{
			name:     "extra nil returns inferred unchanged",
			inferred: []string{"git", "go"},
			extra:    nil,
			dict:     dict,
			want:     []string{"git", "go"},
		},
		{
			name:     "extra empty returns inferred unchanged",
			inferred: []string{"git", "go"},
			extra:    []string{},
			dict:     dict,
			want:     []string{"git", "go"},
		},
		{
			name:     "inferred nil with extra tags",
			inferred: nil,
			extra:    []string{"frond", "workflow"},
			dict:     dict,
			want:     []string{"frond", "workflow"},
		},
		{
			name:     "basic merge no overlap",
			inferred: []string{"git", "go"},
			extra:    []string{"frond", "workflow"},
			dict:     dict,
			want:     []string{"frond", "git", "go", "workflow"},
		},
		{
			name:     "deduplication across sets",
			inferred: []string{"git", "go", "testing"},
			extra:    []string{"git", "frond"},
			dict:     dict,
			want:     []string{"frond", "git", "go", "testing"},
		},
		{
			name:     "case normalization on extra tags",
			inferred: []string{"git"},
			extra:    []string{"Git", "FROND"},
			dict:     dict,
			want:     []string{"frond", "git"},
		},
		{
			name:     "dictionary normalization on extra tags",
			inferred: []string{"go"},
			extra:    []string{"golang"},
			dict:     dict,
			want:     []string{"go"},
		},
		{
			name:     "dictionary normalization prevents phantom duplicates",
			inferred: []string{"go", "testing"},
			extra:    []string{"golang", "py"},
			dict:     dict,
			want:     []string{"go", "python", "testing"},
		},
		{
			name:     "whitespace trimming",
			inferred: []string{"git"},
			extra:    []string{"  frond  ", " workflow "},
			dict:     dict,
			want:     []string{"frond", "git", "workflow"},
		},
		{
			name:     "empty string filtering",
			inferred: []string{"git"},
			extra:    []string{"", "  ", "frond"},
			dict:     dict,
			want:     []string{"frond", "git"},
		},
		{
			name:     "MaxExtraTags cap truncates to 5",
			inferred: []string{"git"},
			extra:    []string{"a", "b", "c", "d", "e", "f", "g"},
			dict:     dict,
			want:     []string{"a", "b", "c", "d", "e", "git"},
		},
		{
			name:     "user tags survive MaxTags cap over inferred",
			inferred: []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8"},
			extra:    []string{"z1", "z2", "z3"},
			dict:     dict,
			want:     []string{"a1", "a2", "a3", "a4", "a5", "z1", "z2", "z3"},
		},
		{
			name:     "sorted output",
			inferred: []string{"z-tag"},
			extra:    []string{"a-tag", "m-tag"},
			dict:     dict,
			want:     []string{"a-tag", "m-tag", "z-tag"},
		},
		{
			name:     "nil dictionary passthrough",
			inferred: []string{"git"},
			extra:    []string{"golang", "frond"},
			dict:     nil,
			want:     []string{"frond", "git", "golang"},
		},
		{
			name:     "duplicate extra tags deduplicated",
			inferred: nil,
			extra:    []string{"frond", "frond", "frond"},
			dict:     dict,
			want:     []string{"frond"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeTags(tt.inferred, tt.extra, tt.dict)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeTags_MaxExtraTags(t *testing.T) {
	dict := NewDictionary()

	// Provide more than MaxExtraTags extra tags
	extra := make([]string, MaxExtraTags+5)
	for i := range extra {
		extra[i] = "tag" + string(rune('a'+i))
	}

	got := MergeTags(nil, extra, dict)

	// Count how many of the extra tags survived — should be at most MaxExtraTags
	if len(got) > MaxExtraTags {
		t.Errorf("MergeTags() returned %d tags from extras alone, want at most %d", len(got), MaxExtraTags)
	}
}

func TestMergeTags_UserTagPriority(t *testing.T) {
	dict := NewDictionary()

	// 5 user tags + 8 inferred tags = 13 total, but MaxTags=8
	// All 5 user tags should survive; only 3 inferred tags fit
	inferred := []string{"i1", "i2", "i3", "i4", "i5", "i6", "i7", "i8"}
	extra := []string{"u1", "u2", "u3", "u4", "u5"}

	got := MergeTags(inferred, extra, dict)

	if len(got) > MaxTags {
		t.Errorf("MergeTags() returned %d tags, want at most %d", len(got), MaxTags)
	}

	// All user tags must be present
	gotSet := make(map[string]bool)
	for _, tag := range got {
		gotSet[tag] = true
	}
	for _, u := range extra {
		if !gotSet[u] {
			t.Errorf("user tag %q was dropped", u)
		}
	}
}
