package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const PolicyPath = "contracts/coverage-policy.json"
const CoveragePath = "contracts/coverage.json"

type Specification struct {
	ID     string `json:"id"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type Policy struct {
	Version              int               `json:"version"`
	DefaultFieldStatus   string            `json:"default_field_status"`
	Specifications       []Specification   `json:"specifications"`
	OperationStatuses    map[string]string `json:"operation_statuses"`
	FieldStatusOverrides map[string]string `json:"field_status_overrides"`
}

type Operation struct {
	Key       string `json:"key"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Operation string `json:"operation_id"`
	Status    string `json:"status"`
}

type Field struct {
	Key           string `json:"key"`
	SourcePointer string `json:"source_pointer"`
	Status        string `json:"status"`
}

type Coverage struct {
	Version        int             `json:"version"`
	Specifications []Specification `json:"specifications"`
	Operations     []Operation     `json:"operations"`
	Fields         []Field         `json:"fields"`
}

func Generate(root, specDir string) (Coverage, error) {
	if specDir == "" {
		return Coverage{}, fmt.Errorf("an external specification directory is required")
	}
	policyBytes, err := os.ReadFile(filepath.Join(root, PolicyPath))
	if err != nil {
		return Coverage{}, err
	}
	var policy Policy
	if err := json.Unmarshal(policyBytes, &policy); err != nil {
		return Coverage{}, fmt.Errorf("decode policy: %w", err)
	}
	if err := validateStatus(policy.DefaultFieldStatus); err != nil {
		return Coverage{}, fmt.Errorf("default field status: %w", err)
	}

	coverage := Coverage{Version: policy.Version}
	usedOperations := map[string]bool{}
	usedFields := map[string]bool{}
	for _, specification := range policy.Specifications {
		if filepath.Base(specification.File) != specification.File {
			return Coverage{}, fmt.Errorf("specification file must be a basename: %s", specification.File)
		}
		documentBytes, err := os.ReadFile(filepath.Join(specDir, specification.File))
		if err != nil {
			return Coverage{}, err
		}
		hash := sha256.Sum256(documentBytes)
		actualHash := hex.EncodeToString(hash[:])
		if actualHash != specification.SHA256 {
			return Coverage{}, fmt.Errorf("%s hash is %s, want %s; review the API contract before updating its hash", specification.File, actualHash, specification.SHA256)
		}
		coverage.Specifications = append(coverage.Specifications, specification)

		var document map[string]any
		if err := json.Unmarshal(documentBytes, &document); err != nil {
			return Coverage{}, fmt.Errorf("decode %s: %w", specification.File, err)
		}
		operations, operationIDs, err := collectOperations(specification.ID, document, policy.OperationStatuses, usedOperations)
		if err != nil {
			return Coverage{}, err
		}
		coverage.Operations = append(coverage.Operations, operations...)
		coverage.Fields = append(coverage.Fields, collectFields(specification.ID, document, operationIDs, policy, usedFields)...)
	}

	for key := range policy.OperationStatuses {
		if !usedOperations[key] {
			return Coverage{}, fmt.Errorf("operation policy %q does not match a specification operation", key)
		}
	}
	for key := range policy.FieldStatusOverrides {
		if !usedFields[key] {
			return Coverage{}, fmt.Errorf("field policy %q does not match a specification field", key)
		}
	}

	sort.Slice(coverage.Specifications, func(i, j int) bool { return coverage.Specifications[i].ID < coverage.Specifications[j].ID })
	sort.Slice(coverage.Operations, func(i, j int) bool { return coverage.Operations[i].Key < coverage.Operations[j].Key })
	sort.Slice(coverage.Fields, func(i, j int) bool { return coverage.Fields[i].Key < coverage.Fields[j].Key })
	return coverage, nil
}

func collectOperations(specID string, document map[string]any, statuses map[string]string, used map[string]bool) ([]Operation, map[string]string, error) {
	paths, _ := document["paths"].(map[string]any)
	result := []Operation{}
	operationIDs := map[string]string{}
	for path, rawPath := range paths {
		pathItem, _ := rawPath.(map[string]any)
		for method, rawOperation := range pathItem {
			if !isHTTPMethod(method) {
				continue
			}
			operationMap, _ := rawOperation.(map[string]any)
			operationID, _ := operationMap["operationId"].(string)
			key := specID + "#" + operationID
			status, ok := statuses[key]
			if !ok {
				return nil, nil, fmt.Errorf("operation %q has no classification", key)
			}
			if err := validateStatus(status); err != nil {
				return nil, nil, fmt.Errorf("operation %q: %w", key, err)
			}
			used[key] = true
			operationIDs[path+"#"+method] = operationID
			result = append(result, Operation{Key: key, Method: strings.ToUpper(method), Path: path, Operation: operationID, Status: status})
		}
	}
	return result, operationIDs, nil
}

func collectFields(specID string, document map[string]any, operationIDs map[string]string, policy Policy, used map[string]bool) []Field {
	result := []Field{}
	var walk func(any, []string)
	walk = func(value any, path []string) {
		switch typed := value.(type) {
		case map[string]any:
			if properties, ok := typed["properties"].(map[string]any); ok {
				propertyNames := make([]string, 0, len(properties))
				for name := range properties {
					propertyNames = append(propertyNames, name)
				}
				sort.Strings(propertyNames)
				for _, name := range propertyNames {
					pointer := jsonPointer(append(append([]string{}, path...), "properties", name))
					key := specID + "#" + pointer
					status := fieldStatus(key, policy, used)
					result = append(result, Field{Key: key, SourcePointer: pointer, Status: status})
				}
			}
			for key, child := range typed {
				walk(child, append(path, key))
			}
		case []any:
			for index, child := range typed {
				walk(child, append(path, fmt.Sprintf("%d", index)))
			}
		}
	}
	walk(document, nil)

	paths, _ := document["paths"].(map[string]any)
	for path, rawPath := range paths {
		pathItem, _ := rawPath.(map[string]any)
		for method, rawOperation := range pathItem {
			if !isHTTPMethod(method) {
				continue
			}
			operationID := operationIDs[path+"#"+method]
			operationMap, _ := rawOperation.(map[string]any)
			parameters, _ := operationMap["parameters"].([]any)
			for index, rawParameter := range parameters {
				parameter, _ := rawParameter.(map[string]any)
				name, _ := parameter["name"].(string)
				location, _ := parameter["in"].(string)
				key := fmt.Sprintf("%s#operation/%s/parameter/%s/%s", specID, operationID, location, name)
				pointer := jsonPointer([]string{"paths", path, method, "parameters", fmt.Sprintf("%d", index)})
				defaultStatus := policy.DefaultFieldStatus
				if policy.OperationStatuses[specID+"#"+operationID] == "implemented" && (location == "path" || (location == "header" && name == "Authorization")) {
					defaultStatus = "implemented"
				}
				result = append(result, Field{Key: key, SourcePointer: pointer, Status: fieldStatusWithDefault(key, defaultStatus, policy, used)})
			}
		}
	}
	return result
}

func fieldStatus(key string, policy Policy, used map[string]bool) string {
	return fieldStatusWithDefault(key, policy.DefaultFieldStatus, policy, used)
}

func fieldStatusWithDefault(key, defaultStatus string, policy Policy, used map[string]bool) string {
	if status, ok := policy.FieldStatusOverrides[key]; ok {
		used[key] = true
		return status
	}
	return defaultStatus
}

func validateStatus(status string) error {
	switch status {
	case "implemented", "sdk-blocked", "intentionally-excluded":
		return nil
	default:
		return fmt.Errorf("invalid status %q", status)
	}
}

func isHTTPMethod(value string) bool {
	switch value {
	case "get", "post", "put", "patch", "delete":
		return true
	default:
		return false
	}
}

func jsonPointer(path []string) string {
	parts := make([]string, 0, len(path))
	for _, part := range path {
		part = strings.ReplaceAll(part, "~", "~0")
		part = strings.ReplaceAll(part, "/", "~1")
		parts = append(parts, part)
	}
	return "/" + strings.Join(parts, "/")
}
