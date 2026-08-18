package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type pluginEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Version     string   `json:"version"`
	Repository  string   `json:"repository"`
	Homepage    string   `json:"homepage"`
	License     string   `json:"license"`
	Tags        []string `json:"tags"`
}

type storeRegistry struct {
	SchemaVersion int           `json:"schema_version"`
	Plugins       []pluginEntry `json:"plugins"`
}

func TestRegistryJSON_SchemaValidation(t *testing.T) {
	data, err := os.ReadFile("registry.json")
	if err != nil {
		t.Fatalf("failed to read registry.json: %v", err)
	}

	var reg storeRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatalf("registry.json is not valid JSON: %v", err)
	}

	if reg.SchemaVersion != 1 {
		t.Errorf("expected schema_version 1, got %d", reg.SchemaVersion)
	}

	if len(reg.Plugins) == 0 {
		t.Fatal("expected at least one plugin in registry.json")
	}

	found := false
	for _, p := range reg.Plugins {
		if p.ID == "antigravity-priority" {
			found = true
			if p.Name == "" {
				t.Error("plugin name must not be empty")
			}
			if p.Description == "" {
				t.Error("plugin description must not be empty")
			}
			if p.Author != "ygq-future" {
				t.Errorf("expected author ygq-future, got %q", p.Author)
			}
			if p.Version == "" {
				t.Error("plugin version must not be empty")
			}
			if p.Repository != "https://github.com/ygq-future/antigravity-priority" {
				t.Errorf("expected repository https://github.com/ygq-future/antigravity-priority, got %q", p.Repository)
			}
			if p.Homepage != "https://github.com/ygq-future/antigravity-priority" {
				t.Errorf("expected homepage https://github.com/ygq-future/antigravity-priority, got %q", p.Homepage)
			}
			if p.License != "MIT" {
				t.Errorf("expected MIT license, got %q", p.License)
			}
			if len(p.Tags) < 3 {
				t.Errorf("expected at least 3 tags, got %v", p.Tags)
			}
		}
	}

	if !found {
		t.Error("plugin id antigravity-priority not found in registry.json")
	}
}

func TestWorkflows_CI_Validation(t *testing.T) {
	ciPath := filepath.Join(".github", "workflows", "ci.yml")
	data, err := os.ReadFile(ciPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", ciPath, err)
	}
	content := string(data)

	expectedSnippets := []string{
		"name: CI",
		"push:",
		"pull_request:",
		"golangci-lint",
		"-race",
		"ubuntu-latest",
		"macos-latest",
		"windows-latest",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(content, snippet) {
			t.Errorf("ci.yml missing required snippet %q", snippet)
		}
	}
}

func TestWorkflows_Release_MatrixValidation(t *testing.T) {
	releasePath := filepath.Join(".github", "workflows", "release.yml")
	data, err := os.ReadFile(releasePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", releasePath, err)
	}
	content := string(data)

	// Verify all 7 target matrix platforms are handled:
	// 1. linux_amd64
	// 2. linux_arm64
	// 3. darwin_arm64
	// 4. darwin_amd64
	// 5. windows_amd64
	// 6. windows_arm64
	// 7. freebsd_amd64
	requiredSnippets := []string{
		"goos: linux",
		"goarch: amd64",
		"goarch: arm64",
		"goos: darwin",
		"windows_amd64",
		"windows_arm64",
		"freebsd_amd64",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(content, snippet) {
			t.Errorf("release.yml missing matrix snippet %q", snippet)
		}
	}

	// Verify required release steps and toolchains
	releaseRequirements := []string{
		"PLUGIN_NAME: antigravity-priority",
		"checksums.txt",
		"sha256sum",
		"vmactions/freebsd-vm",
		"mlugg/setup-zig",
		"msys2/setup-msys2",
	}

	for _, req := range releaseRequirements {
		if !strings.Contains(content, req) {
			t.Errorf("release.yml missing requirement %q", req)
		}
	}
}

func TestDocumentation_BilingualCompleteness(t *testing.T) {
	docFiles := []string{"README.md", "README.en.md"}

	for _, docFile := range docFiles {
		data, err := os.ReadFile(docFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", docFile, err)
		}
		content := string(data)

		// Core sections and standardized CPA paths check
		keywords := []string{
			"antigravity-priority",
			"Urgency",
			"Boost",
			"config",
			"registry.json",
			"/v0/resource/plugins/antigravity-priority/status",
			"/v0/management/plugins/antigravity-priority/run",
			"/v0/management/plugins/antigravity-priority/diagnostics",
			"/v0/management/plugins/antigravity-priority/snapshot/latest",
		}

		for _, kw := range keywords {
			if !strings.Contains(strings.ToLower(content), strings.ToLower(kw)) {
				t.Errorf("%s missing keyword %q", docFile, kw)
			}
		}
	}
}
