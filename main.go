// yaaadabi wires a repository for the develop→review loop: patches
// .claude/settings.local.json, appends the Review loop block to CLAUDE.md,
// creates AGENTS.md if missing, and installs the /task command user-level.
// Run BY THE HUMAN from a terminal — the settings patch is deliberately a
// human act (Claude's auto-mode classifier blocks agents from editing
// permission files, and this tool is not a way around that).
//
// Usage: yaaadabi [-loop-dir DIR] [-codex CMD] [repo]
//
// Two flags, no more. Both name a MACHINE fact the tool cannot guess and a
// human should not have to think about — where this clone lives, and what
// the codex binary is called here. The Loop parameters stay placeholders:
// filling them needs thought, which belongs in CLAUDE.md, not on a command
// line.
//
// Elevation is the consent gate, NOT a capability the work needs: on unix
// the tool gives root away again (see dropPrivileges) before it touches a
// file, and everything below runs as the human who sudo'd.
package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed commands/task.md
var taskCommand []byte

// loopProse names the shared files a loop directory must hold; their
// presence is what makes a directory a yaaadabi clone rather than a guess.
var loopProse = []string{"protocol.md", "developer.md", "reviewer.md"}

// defaultCodex is the reviewer binary assumed when nothing says otherwise —
// the same default protocol.md states for the $CODEX substitution.
const defaultCodex = "codex"

// importRE finds an existing protocol import in a CLAUDE.md, so a repo wired
// to a DIFFERENT clone is reported as such instead of as damage. codexRE
// reads back the machine fact the tool itself wrote, for the same reason.
var (
	importRE = regexp.MustCompile(`(?m)^@(\S*protocol\.md)[ \t]*$`)
	codexRE  = regexp.MustCompile(`(?m)^-[ \t]*Codex command:[ \t]*(\S+)[ \t]*$`)
)

// change is one planned edit. Everything is PLANNED first and applied only
// once every plan succeeded: a repo that gets a permission rule but not the
// CLAUDE.md block it belongs to is worse than a repo the tool refused.
type change struct {
	msg   string
	apply func() error
}

// allowRules is the permission set the loop needs: driving the reviewer, and
// the git verbs step 6 commits with. Everything else runs under the
// session's normal permission mode.
func allowRules(codexCmd string) []string {
	return []string{
		"Bash(" + codexCmd + " exec:*)",
		"Bash(git add:*)",
		"Bash(git commit:*)",
		"Bash(git status:*)",
		"Bash(git diff:*)",
	}
}

// protocolImport is the line that pulls the protocol into a repo's CLAUDE.md.
// It is also the ONLY place the clone's location is recorded: protocol.md
// derives $LOOP from it, so nothing has two paths to keep in sync.
func protocolImport(loopDir string) string {
	return "@" + strings.TrimSuffix(filepath.ToSlash(loopDir), "/") + "/protocol.md"
}

// loopBlock is what gets appended to a repo's CLAUDE.md. The Codex command
// line appears only when it differs from the default, so the common case
// carries no noise.
func loopBlock(loopDir, codexCmd string) string {
	b := "## Review loop\n\n" + protocolImport(loopDir) + "\n\nLoop parameters:\n"
	if codexCmd != defaultCodex {
		b += "- Codex command: " + codexCmd + "\n"
	}
	return b + `- Verify: <build command> && <uncached test command>
- Yardstick docs: <the docs that define "best" for this repo>
- Review focus: <what this codebase is most at risk of>
- Task list: <where [task] findings go — omit this line for TODO.md at the repo root>
`
}

// validateEmitted refuses a value that cannot survive the round trip into a
// shell command and a CLAUDE.md @import. Both substitutions are unquoted by
// contract (protocol.md says so, and a quoted `~` would stop expanding), so
// the safety has to be in the value, not in the quoting. Checked on the
// FINAL string — after `~` handling and after a relative path became
// absolute, since that is what actually gets written.
func validateEmitted(kind, v string) error {
	if v == "" {
		return fmt.Errorf("%s is empty", kind)
	}
	for i, r := range v {
		switch {
		case r <= ' ' || r == 0x7f:
			return fmt.Errorf("%s %q contains whitespace or a control character — it is interpolated unquoted into the review command and into a CLAUDE.md import, so it must not need quoting", kind, v)
		case strings.ContainsRune("\"'`$&;|<>()[]{}*?!#\\^", r):
			return fmt.Errorf("%s %q contains the shell metacharacter %q — it is interpolated unquoted into the review command, so it must not need quoting", kind, v, r)
		case r == '~' && i != 0:
			return fmt.Errorf("%s %q has a `~` that is not the leading one — the shell only expands a leading tilde, so this would be taken literally", kind, v)
		}
	}
	if strings.HasPrefix(v, "~") && !strings.HasPrefix(v, "~/") {
		return fmt.Errorf("%s %q starts with `~` but not `~/` — only the plain home form is supported", kind, v)
	}
	return nil
}

func main() {
	// Defining flags and the usage text touches nothing, so it can happen
	// before the gate — and `-h` should not demand a password.
	loopDirFlag := flag.String("loop-dir", "", "directory holding protocol.md/developer.md/reviewer.md (default: $YAAADABI_DIR, else this executable's own directory)")
	codexFlag := flag.String("codex", defaultCodex, "the codex binary to drive as reviewer")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-loop-dir DIR] [-codex CMD] [repo]\n\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "-help" || arg == "--help" {
			flag.Usage()
			return
		}
	}

	// ---- the only privileged region, and it is three statements long ----
	//
	// Granting permissions must be a human act, enforced OUT of process:
	// require elevation, so a non-elevated agent is refused while a human
	// can supply it (UAC consent, or a sudo password). The CLAUDECODE check
	// is merely a courtesy fast-fail with a clearer message — an agent
	// controls its child environment and can scrub the variable
	// (reviewer-demonstrated), so it is NOT the boundary.
	//
	// Nothing between here and the drop reads, writes, or even names a file:
	// consent is taken from the process token, and privilege is handed back
	// immediately. Everything after this region runs as the invoking human.
	if os.Getenv("CLAUDECODE") != "" {
		fatal("refusing to run inside a Claude Code session — wiring permissions is a human act; run this from your own terminal")
	}
	if !isElevated() {
		fatal("%s", elevationHint)
	}
	report(dropPrivileges(os.Geteuid(), os.Getenv("SUDO_UID"), os.Getenv("SUDO_GID"), os.Getenv("SUDO_USER")))
	// ---------------------- end of privileged region ---------------------

	flag.Parse()
	loopDir, err := resolveLoopDir(*loopDirFlag)
	if err != nil {
		fatal("%v", err)
	}
	codexCmd := *codexFlag
	if err := validateEmitted("codex command", codexCmd); err != nil {
		fatal("%v", err)
	}

	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		fatal("not a directory: %s", dir)
	}
	home, err := invokingHome()
	if err != nil {
		fatal("%v", err)
	}

	fmt.Printf("loop directory: %s\n", loopDir)
	planned, err := planAll(dir, loopDir, codexCmd, home)
	if err != nil {
		fatal("%v", err)
	}
	for _, c := range planned {
		if c.apply != nil {
			if err := c.apply(); err != nil {
				fatal("%v", err)
			}
		}
		fmt.Println(c.msg)
	}

	if _, err := exec.LookPath(codexCmd); err != nil {
		fmt.Printf("WARNING: %q not found on PATH — the loop cannot run without it (if you got here through sudo, sudo may have reset PATH; check it in your own shell)\n", codexCmd)
	}
	fmt.Println("Done. Fill in the <placeholders> in CLAUDE.md's Loop parameters, then start a FRESH session and kick off with /task.")
	fmt.Println("The first session in this repo asks once to approve the external CLAUDE.md import — accept it, or the protocol never loads.")
}

// planAll plans every edit and returns them only if ALL of them succeeded.
// This is the atomicity the tool has: a refusal anywhere means nothing was
// written anywhere, so a repo can never end up with a permission rule for a
// codex binary its CLAUDE.md does not name.
func planAll(dir, loopDir, codexCmd, home string) ([]change, error) {
	plans := []func() (change, error){
		func() (change, error) { return planSettings(dir, codexCmd) },
		func() (change, error) { return planClaudeMD(dir, loopDir, codexCmd) },
		func() (change, error) { return planAgentsMD(dir) },
		func() (change, error) { return planTaskCommand(home) },
	}
	planned := make([]change, 0, len(plans))
	for _, plan := range plans {
		c, err := plan()
		if err != nil {
			return nil, err
		}
		planned = append(planned, c)
	}
	return planned, nil
}

// resolveLoopDir settles where the shared prose lives: the flag, else
// $YAAADABI_DIR, else the directory of the running executable (which is the
// clone itself for the documented `go build -o yaaadabi .` install). The
// result is validated — a directory that does not hold the prose files is a
// mistake worth failing on, not a path to write into a CLAUDE.md.
//
// A leading `~/` is kept verbatim in the written import (Claude Code and the
// shell both expand it, which is what makes a committed CLAUDE.md portable
// between machines) but expanded here for the existence check.
func resolveLoopDir(flagVal string) (string, error) {
	dir, src := flagVal, "-loop-dir"
	if dir == "" {
		dir, src = os.Getenv("YAAADABI_DIR"), "$YAAADABI_DIR"
	}
	if dir == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("cannot locate this executable (%v) — pass -loop-dir", err)
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir, src = filepath.Dir(exe), "this executable's own directory"
	}
	// Absolute FIRST, then validate: a relative path picks up the working
	// directory, and it is the joined result that gets written out.
	check := dir
	if strings.HasPrefix(dir, "~/") {
		home, err := invokingHome()
		if err != nil {
			return "", err
		}
		check = filepath.Join(home, dir[2:])
	} else {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", err
		}
		dir, check = abs, abs
	}
	dir = filepath.ToSlash(dir)
	if len(dir) > 1 {
		dir = strings.TrimSuffix(dir, "/")
	}
	if err := validateEmitted(fmt.Sprintf("loop directory (from %s)", src), dir); err != nil {
		return "", err
	}
	for _, name := range loopProse {
		if _, err := os.Stat(filepath.Join(check, name)); err != nil {
			return "", fmt.Errorf("loop directory %s (from %s) does not contain %s — point -loop-dir or $YAAADABI_DIR at your yaaadabi clone", dir, src, name)
		}
	}
	return dir, nil
}

// refuseSymlink stops the tool from writing through a link it did not
// create. Dropping root reduced this hazard but did not remove it: an agent
// that can create files in the repo can still point CLAUDE.md at any file
// the HUMAN can write — ~/.zshrc, another repo's settings — and a plain
// os.WriteFile follows the link. None of the four paths this tool writes has
// any business being a symlink, so the answer is to refuse, at PLAN time, so
// the refusal happens before anything at all is written.
func refuseSymlink(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil // absent: there is no link to follow
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink — refusing to write through it, because it can point anywhere you can write; if that was deliberate, replace it with a real file", path)
	}
	return nil
}

// refuseSymlinkChain applies that check to every component between a trusted
// base and the target, not just the target itself. Checking only the leaf
// was the same defect one level up: a symlinked `~/.claude` or
// `~/.claude/commands` redirects the MkdirAll and the write inside it just
// as effectively as a symlinked task.md. The base — the repo the human
// named, or their home — is where trust starts and is not itself checked.
func refuseSymlinkChain(base, path string) error {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return refuseSymlink(path) // outside the base: check the target alone
	}
	current := base
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		if err := refuseSymlink(current); err != nil {
			return err
		}
	}
	return nil
}

// planSettings merges the loop's permission rules into
// .claude/settings.local.json, preserving everything already there.
func planSettings(dir, codexCmd string) (change, error) {
	path := filepath.Join(dir, ".claude", "settings.local.json")
	// The .claude directory too: a symlink there redirects the file inside it.
	if err := refuseSymlinkChain(dir, path); err != nil {
		return change{}, err
	}
	root := map[string]any{}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return change{}, fmt.Errorf("cannot read %s: %v", path, err)
	}
	if err == nil {
		if err := json.Unmarshal(raw, &root); err != nil {
			return change{}, fmt.Errorf("%s is not valid JSON, not touching it: %v", path, err)
		}
		if root == nil {
			return change{}, fmt.Errorf("%s: top-level value is null, not an object — fix it by hand", path)
		}
	}
	if v, exists := root["permissions"]; exists {
		if _, ok := v.(map[string]any); !ok {
			return change{}, fmt.Errorf("%s: \"permissions\" is not an object — fix it by hand", path)
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
			return change{}, fmt.Errorf("%s: \"permissions.allow\" is not an array — fix it by hand", path)
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
	for _, rule := range allowRules(codexCmd) {
		if !have[rule] {
			allow = append(allow, rule)
			have[rule] = true
			added++
		}
	}
	if added == 0 {
		return change{msg: path + ": all rules already present"}, nil
	}
	perms["allow"] = allow
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return change{}, err
	}
	return change{
		msg: fmt.Sprintf("%s: added %d rule(s)", path, added),
		apply: func() error {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			return os.WriteFile(path, append(out, '\n'), 0o644)
		},
	}, nil
}

// planClaudeMD appends the Review loop block unless one is already there.
// Both machine facts it writes — the clone's location and the codex command
// — are checked against what the file already says: this tool never guesses
// which of two conflicting values the human meant.
func planClaudeMD(dir, loopDir, codexCmd string) (change, error) {
	path := filepath.Join(dir, "CLAUDE.md")
	if err := refuseSymlinkChain(dir, path); err != nil {
		return change{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return change{}, err
	}
	existing := string(raw)
	importLine := protocolImport(loopDir)
	markers := []string{"## Review loop", importLine, "Loop parameters:"}
	found := 0
	for _, m := range markers {
		if strings.Contains(existing, m) {
			found++
		}
	}
	if found == len(markers) {
		wired := defaultCodex
		if m := codexRE.FindStringSubmatch(existing); m != nil {
			wired = m[1]
		}
		if wired != codexCmd {
			return change{}, fmt.Errorf("%s: already wired for codex command %q, this run specifies %q — the permission rule and the protocol would then name different binaries. Edit the \"Codex command\" line in the Loop parameters block by hand (add it if absent; a value inherited from an imported conventions file is invisible to this tool, and a line in the repo's own block wins over it)", path, wired, codexCmd)
		}
		return change{msg: path + ": already wired"}, nil
	}
	// Wired, but to another clone — the ordinary case after moving machines
	// or moving the clone. Rewriting it silently would be a guess about
	// which path the human wants; say what is there and what this run wanted.
	if !strings.Contains(existing, importLine) {
		if m := importRE.FindStringSubmatch(existing); m != nil {
			return change{}, fmt.Errorf("%s: already wired to a different loop directory (@%s), this run would write %s — update that line by hand, or delete the Review loop block and re-run", path, m[1], importLine)
		}
	}
	if found > 0 {
		return change{}, fmt.Errorf("%s: partially wired (needs the \"## Review loop\" heading, the protocol import line, and a \"Loop parameters:\" block; found %d of 3) — fix it by hand", path, found)
	}
	content := loopBlock(loopDir, codexCmd)
	if existing == "" {
		content = "# CLAUDE.md\n\n" + content
	} else {
		content = strings.TrimRight(existing, "\n") + "\n\n" + content
	}
	return change{
		msg:   path + ": Review loop block appended",
		apply: func() error { return os.WriteFile(path, []byte(content), 0o644) },
	}, nil
}

// planAgentsMD gives Codex-as-reviewer a project entry point if none exists.
func planAgentsMD(dir string) (change, error) {
	path := filepath.Join(dir, "AGENTS.md")
	if err := refuseSymlinkChain(dir, path); err != nil {
		return change{}, err
	}
	if _, err := os.Stat(path); err == nil {
		return change{msg: path + ": exists, untouched"}, nil
	}
	stub := "# AGENTS.md\n\nRead CLAUDE.md for project information; it is the single source of truth.\n"
	return change{
		msg:   path + ": stub created (points at CLAUDE.md)",
		apply: func() error { return os.WriteFile(path, []byte(stub), 0o644) },
	}, nil
}

// planTaskCommand writes the embedded /task command to the user-level
// commands directory. Overwrites: the embedded copy is the source of truth.
// It takes the home directory rather than the command directory so that
// every component it would create — .claude, commands, task.md — is checked
// against the one path the caller actually trusts.
func planTaskCommand(home string) (change, error) {
	cmdDir := filepath.Join(home, ".claude", "commands")
	path := filepath.Join(cmdDir, "task.md")
	if err := refuseSymlinkChain(home, path); err != nil {
		return change{}, err
	}
	if raw, err := os.ReadFile(path); err == nil && string(raw) == string(taskCommand) {
		return change{msg: path + ": up to date"}, nil
	}
	return change{
		msg: path + ": installed",
		apply: func() error {
			if err := os.MkdirAll(cmdDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(path, taskCommand, 0o644)
		},
	}, nil
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
