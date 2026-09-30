package contract

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckedCoverageMatchesExternalSpecifications(t *testing.T) {
	t.Parallel()
	specDir := os.Getenv("LETTERMINT_SPEC_DIR")
	if specDir == "" {
		t.Skip("Set LETTERMINT_SPEC_DIR for external contract checks")
	}

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	actual, err := Generate(root, specDir)
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
		t.Fatal("contracts/coverage.json is stale; run the coverage command with -spec-dir")
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

func TestCheckedCoverageMatchesPolicy(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	var policy Policy
	var coverage Coverage
	for path, target := range map[string]any{PolicyPath: &policy, CoveragePath: &coverage} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
	if coverage.Version != policy.Version || len(coverage.Operations) == 0 || len(coverage.Fields) == 0 || len(coverage.Specifications) != 2 || len(policy.Specifications) != len(coverage.Specifications) {
		t.Fatal("coverage metadata is incomplete")
	}
	for i, spec := range policy.Specifications {
		if coverage.Specifications[i] != spec || filepath.Base(spec.File) != spec.File || len(spec.SHA256) != 64 {
			t.Fatal("specification fingerprints differ from policy")
		}
		if _, err := os.Stat(filepath.Join(root, "contracts", spec.File)); !os.IsNotExist(err) {
			t.Fatal("source specifications must not be stored in the repository")
		}
	}
	operations := map[string]bool{}
	for _, operation := range coverage.Operations {
		if operations[operation.Key] || policy.OperationStatuses[operation.Key] != operation.Status || validateStatus(operation.Status) != nil {
			t.Fatalf("invalid operation classification: %s", operation.Key)
		}
		operations[operation.Key] = true
	}
	if len(operations) != len(policy.OperationStatuses) {
		t.Fatal("operation classifications differ from policy")
	}
	fields := map[string]string{}
	for _, field := range coverage.Fields {
		if _, exists := fields[field.Key]; exists || validateStatus(field.Status) != nil || field.SourcePointer == "" {
			t.Fatalf("invalid field classification: %s", field.Key)
		}
		fields[field.Key] = field.Status
		expected := policy.DefaultFieldStatus
		parts := strings.SplitN(field.Key, "#operation/", 2)
		if len(parts) == 2 {
			parameter := strings.SplitN(parts[1], "/parameter/", 2)
			if len(parameter) == 2 && policy.OperationStatuses[parts[0]+"#"+parameter[0]] == "implemented" && (strings.HasPrefix(parameter[1], "path/") || parameter[1] == "header/Authorization") {
				expected = "implemented"
			}
		}
		if override, ok := policy.FieldStatusOverrides[field.Key]; ok {
			expected = override
		}
		if field.Status != expected {
			t.Fatalf("field classification differs from policy: %s", field.Key)
		}
	}
	for key, status := range policy.FieldStatusOverrides {
		if fields[key] != status {
			t.Fatalf("field classification differs from policy: %s", key)
		}
	}
}

func TestGenerateRequiresExternalSpecificationsAndChecksHash(t *testing.T) {
	t.Parallel()
	root, specDir := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	document := []byte(`{"paths":{"/example":{"get":{"operationId":"example.show"}}},"components":{"schemas":{"Example":{"properties":{"id":{"type":"string"}}}}}}`)
	policy := Policy{
		Version: 1, DefaultFieldStatus: "intentionally-excluded",
		Specifications:    []Specification{{ID: "example", File: "example.json", SHA256: fmt.Sprintf("%x", sha256.Sum256(document))}},
		OperationStatuses: map[string]string{"example#example.show": "implemented"},
	}
	contents, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, PolicyPath), contents, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(root, ""); err == nil {
		t.Fatal("expected missing external directory error")
	}
	if _, err := Generate(root, specDir); err == nil {
		t.Fatal("expected missing specification error")
	}
	path := filepath.Join(specDir, "example.json")
	if err := os.WriteFile(path, document, 0o644); err != nil {
		t.Fatal(err)
	}
	actual, err := Generate(root, specDir)
	if err != nil || len(actual.Operations) != 1 || len(actual.Fields) != 1 {
		t.Fatalf("Generate() = %+v, %v", actual, err)
	}
	if err := os.WriteFile(path, append(document, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(root, specDir); err == nil {
		t.Fatal("expected source fingerprint error")
	}
}
