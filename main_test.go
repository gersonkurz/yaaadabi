package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testLoop is a stand-in clone location for tests that only need the string.
const testLoop = "/opt/loop/yaaadabi"

// loopClone builds a directory that passes resolveLoopDir's validation.
func loopClone(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeClone(t, dir)
	return dir
}

func writeClone(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range loopProse {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// runner returns a helper that carries out a planned change the way main
// does. A closure, so a plan call can be passed straight through as the sole
// argument.
func runner(t *testing.T) func(change, error) string {
	return func(c change, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		if c.apply != nil {
			if err := c.apply(); err != nil {
				t.Fatal(err)
			}
		}
		return c.msg
	}
}

func TestPlanSettingsMergesWithoutDuplicates(t *testing.T) {
	run := runner(t)
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(`{"permissions":{"allow":["Bash(codex exec:*)","Bash(dotnet build:*)"],"deny":["WebFetch"]}}`), 0o644)

	run(planSettings(dir, defaultCodex))
	// Second run must be a no-op.
	msg := run(planSettings(dir, defaultCodex))
	if !strings.Contains(msg, "already present") {
		t.Fatalf("expected idempotent second run, got %q", msg)
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
	for _, want := range append(allowRules(defaultCodex), "Bash(dotnet build:*)") {
		if count[want] != 1 {
			t.Errorf("rule %q: want exactly 1, got %d", want, count[want])
		}
	}
	if len(root.Permissions.Deny) != 1 {
		t.Error("pre-existing deny list was not preserved")
	}
}

// A repo wired for a differently named codex binary must get a rule that
// actually matches the command protocol.md will run.
func TestPlanSettingsUsesConfiguredCodex(t *testing.T) {
	run := runner(t)
	dir := t.TempDir()
	run(planSettings(dir, "/opt/homebrew/bin/codex"))
	raw, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if !strings.Contains(string(raw), `Bash(/opt/homebrew/bin/codex exec:*)`) {
		t.Fatalf("configured codex binary missing from allow rules:\n%s", raw)
	}
	if strings.Contains(string(raw), `Bash(codex exec:*)`) {
		t.Errorf("wrote the default rule as well:\n%s", raw)
	}
}

func TestPlanSettingsRefusesInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("{broken"), 0o644)
	if _, err := planSettings(dir, defaultCodex); err == nil {
		t.Fatal("expected error on invalid JSON, got none")
	}
	if raw, _ := os.ReadFile(path); string(raw) != "{broken" {
		t.Fatal("invalid file was modified")
	}
}

func TestPlanSettingsRefusesUnexpectedShapes(t *testing.T) {
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
		if _, err := planSettings(dir, defaultCodex); err == nil {
			t.Errorf("%s: expected error, got none", name)
		}
		if raw, _ := os.ReadFile(path); string(raw) != content {
			t.Errorf("%s: file was modified", name)
		}
	}
}

func TestPlanClaudeMDRefusesPartialWiring(t *testing.T) {
	importLine := protocolImport(testLoop)
	for name, content := range map[string]string{
		"import line in prose":   "# X\n\nsee " + importLine + " for details\n",
		"heading without import": "# X\n\n## Review loop\n\ntodo\n",
		"markers without params": "# X\n\n## Review loop\n\n" + importLine + "\n",
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(content), 0o644)
		if _, err := planClaudeMD(dir, testLoop, defaultCodex); err == nil {
			t.Errorf("%s: expected error, got none", name)
		}
	}
}

// Moving the clone (or the machine) leaves repos wired to the old path. That
// must be reported as a different loop directory, not appended to blindly and
// not diagnosed as damage.
func TestPlanClaudeMDReportsDifferentLoopDir(t *testing.T) {
	for name, old := range map[string]string{
		"windows clone": "@C:/Projects/yaaadabi/protocol.md",
		"other unix":    "@/srv/shared/yaaadabi/protocol.md",
		"relative":      "@protocol.md",
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "CLAUDE.md"),
			[]byte("# X\n\n## Review loop\n\n"+old+"\n\nLoop parameters:\n- Verify: make\n"), 0o644)
		_, err := planClaudeMD(dir, testLoop, defaultCodex)
		if err == nil {
			t.Errorf("%s: expected error, got none", name)
			continue
		}
		if !strings.Contains(err.Error(), "different loop directory") || !strings.Contains(err.Error(), old[1:]) {
			t.Errorf("%s: unhelpful error: %v", name, err)
		}
		raw, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
		if strings.Contains(string(raw), testLoop) {
			t.Errorf("%s: file was modified", name)
		}
	}
}

func TestPlanClaudeMDAppendsOnceAndCreates(t *testing.T) {
	run := runner(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Existing\n\nproject info\n"), 0o644)

	run(planClaudeMD(dir, testLoop, defaultCodex))
	msg := run(planClaudeMD(dir, testLoop, defaultCodex))
	if !strings.Contains(msg, "already wired") {
		t.Fatalf("expected idempotent second run, got %q", msg)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	s := string(raw)
	importLine := protocolImport(testLoop)
	if !strings.HasPrefix(s, "# Existing") || strings.Count(s, importLine) != 1 || !strings.Contains(s, "- Verify: <build command> && <uncached test command>") {
		t.Fatalf("unexpected CLAUDE.md content:\n%s", s)
	}
	// Every parameter the protocol names must have a line to fill in.
	for _, param := range []string{"- Verify:", "- Yardstick docs:", "- Review focus:", "- Task list:"} {
		if !strings.Contains(s, param) {
			t.Errorf("Loop parameters block lacks %q", param)
		}
	}
	// The default codex command is protocol.md's own default: writing it
	// into every repo would be noise.
	if strings.Contains(s, "Codex command:") {
		t.Errorf("default codex command should not be written:\n%s", s)
	}

	// Missing CLAUDE.md gets created.
	dir2 := t.TempDir()
	run(planClaudeMD(dir2, testLoop, defaultCodex))
	raw2, _ := os.ReadFile(filepath.Join(dir2, "CLAUDE.md"))
	if !strings.Contains(string(raw2), importLine) {
		t.Fatal("created CLAUDE.md lacks import line")
	}
}

// Re-running with a different -codex than the repo is wired for would leave
// the permission rule and the protocol naming different binaries. Both
// directions must be caught, and re-running with the SAME value must not be.
func TestPlanClaudeMDCatchesCodexTransitions(t *testing.T) {
	cases := []struct {
		name        string
		wiredWith   string
		rerunWith   string
		wantRefusal bool
	}{
		{"default then custom", defaultCodex, "codex-nightly", true},
		{"custom then default", "codex-nightly", defaultCodex, true},
		{"custom then other custom", "codex-nightly", "codex-beta", true},
		{"custom then same", "codex-nightly", "codex-nightly", false},
		{"default then same", defaultCodex, defaultCodex, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := runner(t)
			dir := t.TempDir()
			run(planClaudeMD(dir, testLoop, tc.wiredWith))
			before, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))

			c, err := planClaudeMD(dir, testLoop, tc.rerunWith)
			if tc.wantRefusal {
				if err == nil {
					t.Fatalf("expected a refusal, got %q", c.msg)
				}
				if !strings.Contains(err.Error(), tc.wiredWith) || !strings.Contains(err.Error(), tc.rerunWith) {
					t.Errorf("error names neither value clearly: %v", err)
				}
				after, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
				if string(after) != string(before) {
					t.Error("refused plan modified the file")
				}
				return
			}
			if err != nil || !strings.Contains(c.msg, "already wired") {
				t.Fatalf("expected a clean no-op, got %q, %v", c.msg, err)
			}
		})
	}
}

// The refusal must be worth something: nothing at all may be written when
// any part of the plan fails, or the repo keeps a permission rule for a
// binary its CLAUDE.md never names.
func TestPlanAllIsAllOrNothing(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	planned, err := planAll(dir, testLoop, defaultCodex, home)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range planned {
		if c.apply != nil {
			if err := c.apply(); err != nil {
				t.Fatal(err)
			}
		}
	}
	settings := filepath.Join(dir, ".claude", "settings.local.json")
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}

	// Now re-wire with a conflicting codex command: planAll must refuse
	// before anything is applied.
	if _, err := planAll(dir, testLoop, "codex-nightly", home); err == nil {
		t.Fatal("expected planAll to refuse the codex mismatch")
	}
	after, _ := os.ReadFile(settings)
	if string(after) != string(before) {
		t.Error("settings were changed by a plan that failed")
	}
	if strings.Contains(string(after), "codex-nightly") {
		t.Error("permission rule for the rejected binary was written")
	}
}

func TestPlanClaudeMDWritesNonDefaultCodex(t *testing.T) {
	run := runner(t)
	dir := t.TempDir()
	run(planClaudeMD(dir, testLoop, "codex-nightly"))
	raw, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if !strings.Contains(string(raw), "- Codex command: codex-nightly\n") {
		t.Fatalf("non-default codex command not recorded:\n%s", raw)
	}
}

func TestResolveLoopDirPrecedenceAndValidation(t *testing.T) {
	clone := loopClone(t)
	t.Setenv("YAAADABI_DIR", clone)

	// Flag wins over the environment.
	if got, err := resolveLoopDir(loopClone(t)); err != nil || got == filepath.ToSlash(clone) {
		t.Fatalf("flag should win over $YAAADABI_DIR: got %q, %v", got, err)
	}
	// Environment is used when the flag is empty.
	if got, err := resolveLoopDir(""); err != nil || got != filepath.ToSlash(clone) {
		t.Fatalf("env should be used: got %q, %v", got, err)
	}
	// A trailing separator must not produce a doubled slash in the import.
	got, err := resolveLoopDir(clone + "/")
	if err != nil || protocolImport(got) != "@"+filepath.ToSlash(clone)+"/protocol.md" {
		t.Fatalf("trailing slash mishandled: %q -> %q, %v", got, protocolImport(got), err)
	}
	// A directory that is not a clone is a mistake, not a path to write.
	incomplete := t.TempDir()
	os.WriteFile(filepath.Join(incomplete, "protocol.md"), []byte("x"), 0o644)
	if _, err := resolveLoopDir(incomplete); err == nil || !strings.Contains(err.Error(), "developer.md") {
		t.Fatalf("expected a complaint about the missing file, got %v", err)
	}
	if _, err := resolveLoopDir(filepath.Join(clone, "nope")); err == nil {
		t.Fatal("expected an error for a nonexistent directory")
	}
}

// A relative path is only unsafe once it has picked up the working
// directory, so the check has to happen after that, not before.
func TestResolveLoopDirRefusesWhitespaceAfterResolution(t *testing.T) {
	base := t.TempDir()
	clone := filepath.Join(base, "my loop")
	writeClone(t, clone)
	t.Setenv("YAAADABI_DIR", "")

	if _, err := resolveLoopDir(clone); err == nil || !strings.Contains(err.Error(), "whitespace") {
		t.Fatalf("absolute path with a space: expected refusal, got %v", err)
	}
	// The same clone reached by a relative path from inside it.
	t.Chdir(clone)
	if _, err := resolveLoopDir("."); err == nil || !strings.Contains(err.Error(), "whitespace") {
		t.Fatalf("relative path under a whitespace parent: expected refusal, got %v", err)
	}
	// With no flag and no env the fallback is the executable's own
	// directory, not the working directory — so that case is not a
	// whitespace test and is covered by the precedence test instead.
}

func TestValidateEmittedRejectsUnsafeValues(t *testing.T) {
	bad := map[string]string{
		"empty":               "",
		"space":               "/opt/my loop",
		"tab":                 "/opt/loop\tx",
		"newline":             "/opt/loop\nrm -rf /",
		"command sub":         "/opt/$(whoami)/loop",
		"backtick":            "/opt/`id`/loop",
		"semicolon":           "/opt/loop;id",
		"pipe":                "/opt/loop|id",
		"ampersand":           "/opt/loop&",
		"glob":                "/opt/loop*",
		"question":            "/opt/loop?",
		"brace":               "/opt/{a,b}/loop",
		"bracket":             "/opt/[ab]/loop",
		"redirect":            "/opt/loop>x",
		"quote":               "/opt/\"loop\"",
		"backslash":           "/opt/lo\\op",
		"comment":             "/opt/loop#x",
		"interior tilde":      "/opt/~loop",
		"tilde not home":      "~user/loop",
		"codex with args":     "codex --yolo",
		"codex with subshell": "codex$(id)",
	}
	for name, v := range bad {
		if err := validateEmitted("value", v); err == nil {
			t.Errorf("%s (%q): expected refusal, got none", name, v)
		}
	}
	good := []string{"/Users/x/dev/yaaadabi", "~/dev/yaaadabi", "C:/Projects/yaaadabi", "codex", "codex-nightly", "/opt/homebrew/bin/codex", "/opt/münchen/yaaadabi", "/opt/loop-1.2+3", "/"}
	for _, v := range good {
		if err := validateEmitted("value", v); err != nil {
			t.Errorf("%q should be accepted: %v", v, err)
		}
	}
}

// The root directory is a degenerate but legal clone location; the import
// must not come out as "@//protocol.md".
func TestProtocolImportHasNoDoubleSlash(t *testing.T) {
	for dir, want := range map[string]string{
		"/":                    "@/protocol.md",
		"/opt/loop":            "@/opt/loop/protocol.md",
		"/opt/loop/":           "@/opt/loop/protocol.md",
		"~/dev/yaaadabi":       "@~/dev/yaaadabi/protocol.md",
		"C:/Projects/yaaadabi": "@C:/Projects/yaaadabi/protocol.md",
	} {
		if got := protocolImport(dir); got != want {
			t.Errorf("protocolImport(%q) = %q, want %q", dir, got, want)
		}
	}
}

// planTaskCommand must create the whole ~/.claude/commands path when the
// machine has never had one — the case on a fresh install.
func TestPlanTaskCommandCreatesTree(t *testing.T) {
	run := runner(t)
	home := t.TempDir()
	cmdDir := filepath.Join(home, ".claude", "commands")
	if msg := run(planTaskCommand(home)); !strings.Contains(msg, "installed") {
		t.Fatalf("first install: %q", msg)
	}
	raw, err := os.ReadFile(filepath.Join(cmdDir, "task.md"))
	if err != nil || string(raw) != string(taskCommand) {
		t.Fatalf("installed copy differs from the embedded source: %v", err)
	}
	if msg := run(planTaskCommand(home)); !strings.Contains(msg, "up to date") {
		t.Fatalf("second install should be a no-op: %q", msg)
	}
	// A stale copy is replaced: the embedded file is the source of truth.
	os.WriteFile(filepath.Join(cmdDir, "task.md"), []byte("old"), 0o644)
	if msg := run(planTaskCommand(home)); !strings.Contains(msg, "installed") {
		t.Fatalf("stale copy not refreshed: %q", msg)
	}
}

// Dropping root lowered the symlink hazard from "any file on the machine" to
// "any file the human can write" — which is still their ~/.zshrc. None of
// the paths this tool writes may be followed through a link, and the refusal
// has to land at plan time, before anything is written at all.
func TestPlanRefusesToWriteThroughSymlinks(t *testing.T) {
	canaryText := "PRECIOUS\n"
	cases := map[string]func(t *testing.T, repo, canary string) (change, error){
		"CLAUDE.md": func(t *testing.T, repo, canary string) (change, error) {
			link(t, filepath.Join(repo, "CLAUDE.md"), canary)
			return planClaudeMD(repo, testLoop, defaultCodex)
		},
		"AGENTS.md": func(t *testing.T, repo, canary string) (change, error) {
			link(t, filepath.Join(repo, "AGENTS.md"), canary)
			return planAgentsMD(repo)
		},
		"settings.local.json": func(t *testing.T, repo, canary string) (change, error) {
			os.MkdirAll(filepath.Join(repo, ".claude"), 0o755)
			link(t, filepath.Join(repo, ".claude", "settings.local.json"), canary)
			return planSettings(repo, defaultCodex)
		},
		".claude directory": func(t *testing.T, repo, canary string) (change, error) {
			link(t, filepath.Join(repo, ".claude"), filepath.Dir(canary))
			return planSettings(repo, defaultCodex)
		},
		"task.md": func(t *testing.T, home, canary string) (change, error) {
			cmdDir := filepath.Join(home, ".claude", "commands")
			os.MkdirAll(cmdDir, 0o755)
			link(t, filepath.Join(cmdDir, "task.md"), canary)
			return planTaskCommand(home)
		},
		// The parents matter as much as the leaf: a symlinked .claude or
		// .claude/commands redirects the MkdirAll and the write inside it.
		"~/.claude directory": func(t *testing.T, home, canary string) (change, error) {
			link(t, filepath.Join(home, ".claude"), filepath.Dir(canary))
			return planTaskCommand(home)
		},
		"~/.claude/commands directory": func(t *testing.T, home, canary string) (change, error) {
			os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
			link(t, filepath.Join(home, ".claude", "commands"), filepath.Dir(canary))
			return planTaskCommand(home)
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			repo := t.TempDir()
			canary := filepath.Join(t.TempDir(), "canary")
			if err := os.WriteFile(canary, []byte(canaryText), 0o644); err != nil {
				t.Fatal(err)
			}
			c, err := setup(t, repo, canary)
			if err == nil {
				t.Fatalf("expected a refusal, got plan %q", c.msg)
			}
			if !strings.Contains(err.Error(), "symlink") {
				t.Errorf("error should name the symlink: %v", err)
			}
			if raw, _ := os.ReadFile(canary); string(raw) != canaryText {
				t.Errorf("the link target was written through: %q", raw)
			}
		})
	}
}

// The refusal must also stop the run as a whole: a repo that gets its
// permission rule while its CLAUDE.md is a hostile symlink is the bad case.
func TestPlanAllRefusesSymlinkedTarget(t *testing.T) {
	repo := t.TempDir()
	canary := filepath.Join(t.TempDir(), "canary")
	os.WriteFile(canary, []byte("PRECIOUS\n"), 0o644)
	link(t, filepath.Join(repo, "CLAUDE.md"), canary)

	if _, err := planAll(repo, testLoop, defaultCodex, t.TempDir()); err == nil {
		t.Fatal("expected planAll to refuse")
	}
	if _, err := os.Stat(filepath.Join(repo, ".claude", "settings.local.json")); err == nil {
		t.Error("settings were written despite the refusal")
	}
	if raw, _ := os.ReadFile(canary); string(raw) != "PRECIOUS\n" {
		t.Errorf("link target was written through: %q", raw)
	}
}

func link(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Symlink(to, from); err != nil {
		t.Fatal(err)
	}
}
