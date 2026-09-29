package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckedCoverageMatchesOpenAPISnapshots(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	actual, err := Generate(root)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	want, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		t.Fatalf("marshal coverage: %v", err)
	}
	want = append(want, '\n')
	checked, err := os.ReadFile(filepath.Join(root, CoveragePath))
	if err != nil {
		t.Fatalf("read coverage: %v", err)
	}
	if !bytes.Equal(checked, want) {
		t.Fatal("contracts/coverage.json is stale; run go generate ./...")
	}

	if len(actual.Operations) == 0 || len(actual.Fields) == 0 {
		t.Fatal("coverage must contain operations and fields")
	}
	for _, operation := range actual.Operations {
		if err := validateStatus(operation.Status); err != nil {
			t.Fatalf("operation %s: %v", operation.Key, err)
		}
	}
	for _, field := range actual.Fields {
		if err := validateStatus(field.Status); err != nil {
			t.Fatalf("field %s: %v", field.Key, err)
		}
	}
}
