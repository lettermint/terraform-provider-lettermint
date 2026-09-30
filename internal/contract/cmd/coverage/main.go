package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lettermint/terraform-provider-lettermint/internal/contract"
)

func main() {
	specDir := flag.String("spec-dir", "", "External directory with the API specifications")
	check := flag.Bool("check", false, "Check the coverage report without writing files")
	flag.Parse()
	if *specDir == "" {
		fmt.Fprintln(os.Stderr, "Pass -spec-dir with the external API specification directory.")
		os.Exit(2)
	}
	root, err := filepath.Abs(".")
	if err != nil {
		panic(err)
	}
	coverage, err := contract.Generate(root, *specDir)
	if err != nil {
		panic(err)
	}
	contents, err := json.MarshalIndent(coverage, "", "  ")
	if err != nil {
		panic(err)
	}
	contents = append(contents, '\n')
	if *check {
		checked, err := os.ReadFile(filepath.Join(root, contract.CoveragePath))
		if err != nil || !bytes.Equal(checked, contents) {
			fmt.Fprintln(os.Stderr, "contracts/coverage.json differs from the external specifications")
			os.Exit(1)
		}
		fmt.Println("Coverage matches the external specifications.")
		return
	}
	if err := os.WriteFile(filepath.Join(root, contract.CoveragePath), contents, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s\n", contract.CoveragePath)
}
