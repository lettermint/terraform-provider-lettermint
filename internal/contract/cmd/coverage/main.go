package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lettermint/terraform-provider-lettermint/internal/contract"
)

func main() {
	root, err := filepath.Abs(".")
	if err != nil {
		panic(err)
	}
	coverage, err := contract.Generate(root)
	if err != nil {
		panic(err)
	}
	contents, err := json.MarshalIndent(coverage, "", "  ")
	if err != nil {
		panic(err)
	}
	contents = append(contents, '\n')
	if err := os.WriteFile(filepath.Join(root, contract.CoveragePath), contents, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s\n", contract.CoveragePath)
}
