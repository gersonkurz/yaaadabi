package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureSettingsMergesWithoutDuplicates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"permissions":{"allow":["Bash(codex exec:*)","Bash(dotnet build:*)"],"deny":["WebFetch"]}}`), 0o644)

	if _, err := ensureSettings(dir); err != nil {
		t.Fatal(err)
	}
	// Second run must be a no-op.
	msg, err := ensureSettings(dir)
	if err != nil || !strings.Contains(msg, "already present") {
		t.Fatalf("expected idempotent second run, got %q, %v", msg, err)
	}

	raw, _ := os.ReadFile(path)
	var root struct {
		Permissions struct {
			Allow []string `json:"allow"`
			Deny  []string `json:"deny"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	count := map[string]int{}
	for _, r := range root.Permissions.Allow {
		count[r]++
	}
	for _, want := range append(baseAllow, "Bash(dotnet build:*)") {
		if count[want] != 1 {
			t.Errorf("rule %q: want exactly 1, got %d", want, count[want])
		}
	}
	if len(root.Permissions.Deny) != 1 {
		t.Error("pre-existing deny list was not preserved")
	}
}

func TestEnsureSettingsRefusesInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("{broken"), 0o644)
	if _, err := ensureSettings(dir); err == nil {
		t.Fatal("expected error on invalid JSON, got none")
	}
	if raw, _ := os.ReadFile(path); string(raw) != "{broken" {
		t.Fatal("invalid file was modified")
	}
}

func TestEnsureSettingsRefusesUnexpectedShapes(t *testing.T) {
	cases := map[string]string{
		"top-level null":     `null`,
		"permissions string": `{"permissions":"nope"}`,
		"allow object":       `{"permissions":{"allow":{"x":1}}}`,
	}
	for name, content := range cases {
		dir := t.TempDir()
		path := filepath.Join(dir, ".claude", "settings.local.json")
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(content), 0o644)
		if _, err := ensureSettings(dir); err == nil {
			t.Errorf("%s: expected error, got none", name)
		}
		if raw, _ := os.ReadFile(path); string(raw) != content {
			t.Errorf("%s: file was modified", name)
		}
	}
}

func TestEnsureClaudeMDRefusesPartialWiring(t *testing.T) {
	for name, content := range map[string]string{
		"import line in prose":   "# X\n\nsee " + importLine + " for details\n",
		"heading without import": "# X\n\n## Review loop\n\ntodo\n",
		"markers without params": "# X\n\n## Review loop\n\n" + importLine + "\n",
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(content), 0o644)
		if _, err := ensureClaudeMD(dir); err == nil {
			t.Errorf("%s: expected error, got none", name)
		}
	}
}

func TestEnsureClaudeMDAppendsOnceAndCreates(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Existing\n\nproject info\n"), 0o644)

	if _, err := ensureClaudeMD(dir); err != nil {
		t.Fatal(err)
	}
	msg, err := ensureClaudeMD(dir)
	if err != nil || !strings.Contains(msg, "already wired") {
		t.Fatalf("expected idempotent second run, got %q, %v", msg, err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	s := string(raw)
	if !strings.HasPrefix(s, "# Existing") || strings.Count(s, importLine) != 1 || !strings.Contains(s, "- Verify: <build command> && <uncached test command>") {
		t.Fatalf("unexpected CLAUDE.md content:\n%s", s)
	}

	// Missing CLAUDE.md gets created.
	dir2 := t.TempDir()
	if _, err := ensureClaudeMD(dir2); err != nil {
		t.Fatal(err)
	}
	raw2, _ := os.ReadFile(filepath.Join(dir2, "CLAUDE.md"))
	if !strings.Contains(string(raw2), importLine) {
		t.Fatal("created CLAUDE.md lacks import line")
	}
}
