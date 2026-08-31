// yaaadabi wires a repository for the develop→review loop: patches
// .claude/settings.local.json, appends the Review loop block to CLAUDE.md,
// creates AGENTS.md if missing, and installs the /task command user-level.
// Run BY THE HUMAN from a terminal — the settings patch is deliberately a
// human act (Claude's auto-mode classifier blocks agents from editing
// permission files, and this tool is not a way around that).
//
// Usage: yaaadabi [-verify "just build && just selftest"] [-allow "Bash(just:*)"]... [dir]
package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
- Verify: %s
- Yardstick docs: <the docs that define "best" for this repo>
- Review focus: <what this codebase is most at risk of>
`

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	verify := flag.String("verify", "<build command> && <test command>", "Verify line for the Loop parameters block")
	var extraAllow stringList
	flag.Var(&extraAllow, "allow", "extra permission rule (repeatable), e.g. \"Bash(just:*)\"")
	flag.Parse()

	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		fatal("not a directory: %s", dir)
	}

	report(ensureSettings(dir, extraAllow))
	report(ensureClaudeMD(dir, *verify))
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
func ensureSettings(dir string, extra []string) (string, error) {
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
	for _, rule := range append(append([]string{}, baseAllow...), extra...) {
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
func ensureClaudeMD(dir, verify string) (string, error) {
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
	block := fmt.Sprintf(loopBlock, verify)
	content := string(existing)
	if content == "" {
		content = "# CLAUDE.md\n\n" + block
	} else {
		content = strings.TrimRight(content, "\n") + "\n\n" + block
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
