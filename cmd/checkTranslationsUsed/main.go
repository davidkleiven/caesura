package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: go run . <translation_file> <root_folder>")
		os.Exit(1)
	}

	translationFile := os.Args[1]
	rootFolder := os.Args[2]

	// Verify translation file exists
	if _, err := os.Stat(translationFile); os.IsNotExist(err) {
		log.Fatalf("Translation file not found: %v", err)
	}

	var mapping map[string]map[string]string
	content, err := os.ReadFile(translationFile)
	if err != nil {
		log.Fatalf("Failed to read translation file: %v", err)
	}

	err = yaml.Unmarshal(content, &mapping)
	if err != nil {
		log.Fatalf("Failed to parse translation file: %v", err)
	}

	// Verify root folder exists
	if _, err := os.Stat(rootFolder); os.IsNotExist(err) {
		log.Fatalf("Root folder not found: %v", err)
	}

	used := make(map[string]bool)
	for _, item := range mapping {
		for k := range item {
			used[k] = false
		}
	}

	// Walk through all files in root folder
	err = filepath.WalkDir(rootFolder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Warning: Could not read file %s: %v", path, err)
			return nil
		}

		fileContent := string(data)
		for k := range used {
			if strings.Contains(fileContent, "\""+k+"\"") {
				used[k] = true
			}
		}
		return nil
	})

	unusedKeys := make([]string, 0, len(used))
	for k, v := range used {
		if !v {
			unusedKeys = append(unusedKeys, k)
		}
	}

	if len(unusedKeys) > 0 {
		fmt.Printf("The following translation keys are not used: %v\n", unusedKeys)
		fmt.Printf("Total unused keys: %d\n", len(unusedKeys))
		os.Exit(1)
	}

	fmt.Printf("✅ All %d translation keys are being used!\n", len(used))
}
