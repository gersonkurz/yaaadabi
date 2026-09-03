// yaaadabi wires a repository for the develop→review loop: patches
// .claude/settings.local.json, appends the Review loop block to CLAUDE.md,
// creates AGENTS.md if missing, and installs the /task command user-level.
// Run BY THE HUMAN from a terminal — the settings patch is deliberately a
// human act (Claude's auto-mode classifier blocks agents from editing
// permission files, and this tool is not a way around that).
//
// Usage: yaaadabi [dir]
//
// No flags: the permission rules are a fixed set (codex + git — everything
// else runs under the session's normal permission mode), and the Loop
// parameters are written as placeholders because filling them needs human
// thought, which belongs in CLAUDE.md, not on a command line.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

//go:embed commands/task.md
var taskCommand []byte

// ponytail: absolute clone path hardcoded, matches protocol.md's own
// references; make it a flag if a machine ever uses a different layout.
const importLine = "@C:/Projects/yaaadabi/protocol.md"

var baseAllow = []string{
	"Bash(codex exec:*)",
	"Bash(git add:*)",
	"Bash(git commit:*)",
	"Bash(git status:*)",
	"Bash(git diff:*)",
}

const loopBlock = `## Review loop

` + importLine + `

Loop parameters:
- Verify: <build command> && <uncached test command>
- Yardstick docs: <the docs that define "best" for this repo>
- Review focus: <what this codebase is most at risk of>
- Task list: <where [task] findings go — omit this line for TODO.md at the repo root>
`

func main() {
	// Granting permissions must be a human act, enforced OUT of process:
	// require elevation, so a non-elevated agent is refused while a human
	// can run it from an elevated shell (UAC consent already given). The
	// CLAUDECODE check is merely a courtesy fast-fail with a clearer
	// message — an agent controls its child environment and can scrub the
	// variable (reviewer-demonstrated), so it is NOT the boundary.
	if os.Getenv("CLAUDECODE") != "" {
		fatal("refusing to run inside a Claude Code session — wiring permissions is a human act; run this from your own (elevated) terminal")
	}
	if !isElevated() {
		fatal("administrator required: run this from an elevated (administrator) shell — elevation is the out-of-process human consent this tool requires")
	}

	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		fatal("not a directory: %s", dir)
	}

	report(ensureSettings(dir))
	report(ensureClaudeMD(dir))
	report(ensureAgentsMD(dir))

	home, err := os.UserHomeDir()
	if err != nil {
		fatal("cannot resolve home directory: %v", err)
	}
	report(installTaskCommand(filepath.Join(home, ".claude", "commands")))

	if _, err := exec.LookPath("codex"); err != nil {
		fmt.Println("WARNING: codex not found on PATH — the loop cannot run without it")
	}
	fmt.Println("Done. Fill in the <placeholders> in CLAUDE.md's Loop parameters, then start a FRESH session and kick off with /task.")
}

// ensureSettings merges the loop's permission rules into
// .claude/settings.local.json, preserving everything already there.
func ensureSettings(dir string) (string, error) {
	path := filepath.Join(dir, ".claude", "settings.local.json")
	root := map[string]any{}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("cannot read %s: %v", path, err)
	}
	if err == nil {
		if err := json.Unmarshal(raw, &root); err != nil {
			return "", fmt.Errorf("%s is not valid JSON, not touching it: %v", path, err)
		}
		if root == nil {
			return "", fmt.Errorf("%s: top-level value is null, not an object — fix it by hand", path)
		}
	}
	if v, exists := root["permissions"]; exists {
		if _, ok := v.(map[string]any); !ok {
			return "", fmt.Errorf("%s: \"permissions\" is not an object — fix it by hand", path)
		}
	}
	perms, _ := root["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
		root["permissions"] = perms
	}
	var allow []any
	if v, exists := perms["allow"]; exists {
		a, ok := v.([]any)
		if !ok {
			return "", fmt.Errorf("%s: \"permissions.allow\" is not an array — fix it by hand", path)
		}
		allow = a
	}
	have := map[string]bool{}
	for _, v := range allow {
		if s, ok := v.(string); ok {
			have[s] = true
		}
	}
	added := 0
	for _, rule := range baseAllow {
		if !have[rule] {
			allow = append(allow, rule)
			have[rule] = true
			added++
		}
	}
	if added == 0 {
		return path + ": all rules already present", nil
	}
	perms["allow"] = allow
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s: added %d rule(s)", path, added), nil
}

// ensureClaudeMD appends the Review loop block unless one is already there.
func ensureClaudeMD(dir string) (string, error) {
	path := filepath.Join(dir, "CLAUDE.md")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	markers := []string{"## Review loop", importLine, "Loop parameters:"}
	found := 0
	for _, m := range markers {
		if strings.Contains(string(existing), m) {
			found++
		}
	}
	if found == len(markers) {
		return path + ": already wired", nil
	}
	if found > 0 {
		return "", fmt.Errorf("%s: partially wired (needs the \"## Review loop\" heading, the protocol import line, and a \"Loop parameters:\" block; found %d of 3) — fix it by hand", path, found)
	}
	content := string(existing)
	if content == "" {
		content = "# CLAUDE.md\n\n" + loopBlock
	} else {
		content = strings.TrimRight(content, "\n") + "\n\n" + loopBlock
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path + ": Review loop block appended", nil
}

// ensureAgentsMD gives Codex-as-reviewer a project entry point if none exists.
func ensureAgentsMD(dir string) (string, error) {
	path := filepath.Join(dir, "AGENTS.md")
	if _, err := os.Stat(path); err == nil {
		return path + ": exists, untouched", nil
	}
	stub := "# AGENTS.md\n\nRead CLAUDE.md for project information; it is the single source of truth.\n"
	if err := os.WriteFile(path, []byte(stub), 0o644); err != nil {
		return "", err
	}
	return path + ": stub created (points at CLAUDE.md)", nil
}

// isElevated reports whether the process holds an elevated token, queried
// in-process from the Windows API — nothing PATH- or env-resolvable to
// spoof (a subprocess check was reviewer-bypassed with a fake net.exe).
func isElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// installTaskCommand writes the embedded /task command to the user-level
// commands directory. Overwrites: the embedded copy is the source of truth.
func installTaskCommand(cmdDir string) (string, error) {
	path := filepath.Join(cmdDir, "task.md")
	if raw, err := os.ReadFile(path); err == nil && string(raw) == string(taskCommand) {
		return path + ": up to date", nil
	}
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, taskCommand, 0o644); err != nil {
		return "", err
	}
	return path + ": installed", nil
}

func report(msg string, err error) {
	if err != nil {
		fatal("%v", err)
	}
	fmt.Println(msg)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "yaaadabi: "+format+"\n", args...)
	os.Exit(1)
}
