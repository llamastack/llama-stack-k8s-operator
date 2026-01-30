/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package main provides the generate-config binary for merging external provider
// configurations into the LlamaStack config.yaml at pod startup.
//
// This binary is used as an init container to:
//  1. Load the base config.yaml (from distribution or user ConfigMap)
//  2. Read provider metadata from all installed external providers
//  3. Merge external provider entries into the config
//  4. Write the final merged config.yaml for the main container
//
// Usage:
//
//	generate-config --metadata-dir /path/to/metadata \
//	                --base-config /path/to/base/config.yaml \
//	                --output /path/to/output/config.yaml
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/llamastack/llama-stack-k8s-operator/pkg/deploy"
)

// Version is set during build via ldflags.
var Version = "dev"

// configFilePermissions is the permission mode for config files.
const configFilePermissions = 0o600

// Config holds the CLI configuration.
type Config struct {
	MetadataDir    string
	BaseConfigPath string
	OutputPath     string
	Version        bool
}

func main() {
	cfg := parseFlags()

	if cfg.Version {
		fmt.Printf("generate-config version %s\n", Version)
		os.Exit(0)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() Config {
	var cfg Config
	flag.StringVar(&cfg.MetadataDir, "metadata-dir", "", "Directory containing provider metadata subdirectories")
	flag.StringVar(&cfg.BaseConfigPath, "base-config", "", "Path to the base config.yaml file")
	flag.StringVar(&cfg.OutputPath, "output", "", "Path where the merged config.yaml will be written")
	flag.BoolVar(&cfg.Version, "version", false, "Print version and exit")
	flag.Parse()
	return cfg
}

func run(cfg Config) error {
	if err := validateFlags(cfg); err != nil {
		return err
	}

	fmt.Printf("Generating LlamaStack config.yaml...\n")
	fmt.Printf("  Base config: %s\n", cfg.BaseConfigPath)
	fmt.Printf("  Metadata dir: %s\n", cfg.MetadataDir)
	fmt.Printf("  Output: %s\n", cfg.OutputPath)
	fmt.Println()

	return generateConfig(cfg)
}

func validateFlags(cfg Config) error {
	if cfg.MetadataDir == "" {
		return errors.New("failed to validate flags: --metadata-dir is required")
	}
	if cfg.BaseConfigPath == "" {
		return errors.New("failed to validate flags: --base-config is required")
	}
	if cfg.OutputPath == "" {
		return errors.New("failed to validate flags: --output is required")
	}
	return nil
}

func generateConfig(cfg Config) error {
	// Check if metadata directory exists
	if _, err := os.Stat(cfg.MetadataDir); os.IsNotExist(err) {
		fmt.Println("No external providers detected (metadata directory does not exist)")
		return copyFile(cfg.BaseConfigPath, cfg.OutputPath)
	}

	// Check if metadata directory is empty
	entries, err := os.ReadDir(cfg.MetadataDir)
	if err != nil {
		return fmt.Errorf("failed to read metadata directory: %w", err)
	}

	// Count subdirectories (each represents a provider)
	providerCount := countProviders(entries)
	if providerCount == 0 {
		fmt.Println("No external providers detected (metadata directory is empty)")
		return copyFile(cfg.BaseConfigPath, cfg.OutputPath)
	}

	fmt.Printf("Found %d external provider(s) to merge\n", providerCount)

	// Perform the config generation
	warnings, err := deploy.GenerateConfig(cfg.BaseConfigPath, cfg.MetadataDir, cfg.OutputPath)
	if err != nil {
		return err
	}

	// Print warnings
	for _, w := range warnings {
		fmt.Printf("\nWARNING: %s\n", w.Message)
	}

	fmt.Printf("\nSuccessfully generated config at: %s\n", cfg.OutputPath)
	return nil
}

func countProviders(entries []os.DirEntry) int {
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			count++
		}
	}
	return count
}

// copyFile copies the source file to destination without modification.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src) //nolint:gosec // src is a trusted path from CLI flag
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(dst, data, configFilePermissions); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	fmt.Printf("Copied base config to: %s\n", dst)
	return nil
}
