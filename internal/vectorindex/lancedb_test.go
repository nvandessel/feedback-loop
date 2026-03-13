package vectorindex

import "testing"

func TestLanceDBIndex_Create(t *testing.T) {
	dir := t.TempDir()
	idx, err := NewLanceDBIndex(LanceDBConfig{Dir: dir, Dims: 8})
	if err != nil {
		t.Fatalf("NewLanceDBIndex: %v", err)
	}
	defer idx.Close()
	if idx.Len() != 0 {
		t.Errorf("expected Len()=0, got %d", idx.Len())
	}
}
