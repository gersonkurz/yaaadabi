//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/user"
	"slices"
	"strconv"
	"syscall"
)

// elevationHint names the platform's way of supplying the consent this tool
// requires. macOS/Linux have no UAC; sudo's password prompt is the closest
// equivalent that an agent cannot satisfy on its own.
const elevationHint = "run this from your own terminal with sudo (`sudo yaaadabi <repo>`) — sudo's password prompt is the out-of-process human consent this tool requires"

// isElevated reports whether the process runs as root, queried in-process
// from the kernel — nothing PATH- or env-resolvable to spoof (a subprocess
// check was reviewer-bypassed on Windows with a fake net.exe).
//
// Weaker than Windows UAC in one respect, and the README says so: sudo
// caches its authentication for a few minutes, so an agent that runs
// straight after the human sudo'd can ride that timestamp without knowing
// the password. `Defaults timestamp_timeout=0` in sudoers closes it.
func isElevated() bool { return os.Geteuid() == 0 }

type dropMode int

const (
	dropNotNeeded  dropMode = iota // not elevated: already the human
	dropToUser                     // via sudo: become the human again
	dropImpossible                 // root, but no sudo: nobody to become
)

// dropTarget decides who this process must become, separated from the
// syscalls so the decision is testable without root.
func dropTarget(euid int, sudoUID, sudoGID string) (dropMode, int, int, error) {
	if euid != 0 {
		return dropNotNeeded, -1, -1, nil
	}
	if sudoUID == "" && sudoGID == "" {
		return dropImpossible, -1, -1, nil
	}
	uid, uerr := strconv.Atoi(sudoUID)
	gid, gerr := strconv.Atoi(sudoGID)
	if uerr != nil || gerr != nil {
		return 0, -1, -1, fmt.Errorf("SUDO_UID/SUDO_GID are set but not numeric (%q/%q) — refusing to write anything as root", sudoUID, sudoGID)
	}
	if uid <= 0 || gid < 0 {
		return 0, -1, -1, fmt.Errorf("SUDO_UID/SUDO_GID name root or a negative id (%d/%d) — refusing to write anything as root", uid, gid)
	}
	return dropToUser, uid, gid, nil
}

// invokingGroups resolves the human's REAL group list, so that dropping
// privileges restores the access they actually have rather than a subset of
// it. Keeping only the primary group would make a repo that is writable
// through a supplementary group (a shared project group, admin, staff)
// unwritable to the tool while remaining writable to the human who ran it.
//
// Falls back to the primary group with a warning rather than failing: a
// wrong group list can only cause a visible "permission denied", never
// silent damage, and a tool that refuses to run because a directory service
// is slow is worse than one that says why it might fail.
func invokingGroups(sudoUser string, uid, gid int) ([]int, string) {
	if u, err := lookupInvoking(sudoUser, uid); err == nil {
		if ids, err := u.GroupIds(); err == nil && len(ids) > 0 {
			groups := make([]int, 0, len(ids)+1)
			for _, id := range ids {
				if n, err := strconv.Atoi(id); err == nil {
					groups = append(groups, n)
				}
			}
			if len(groups) > 0 {
				if !slices.Contains(groups, gid) {
					groups = append(groups, gid)
				}
				return groups, ""
			}
		}
	}
	who := sudoUser
	if who == "" {
		who = "uid " + strconv.Itoa(uid)
	}
	return []int{gid}, fmt.Sprintf("WARNING: could not resolve %s's supplementary groups; continuing with the primary group only — if wiring then fails with \"permission denied\" on a repo you can write yourself, that is why", who)
}

// lookupInvoking finds the account behind sudo, by name where sudo recorded
// one and by uid otherwise.
func lookupInvoking(sudoUser string, uid int) (*user.User, error) {
	if sudoUser != "" && sudoUser != "root" {
		return user.Lookup(sudoUser)
	}
	return user.LookupId(strconv.Itoa(uid))
}

// dropPrivileges gives root away again, permanently, BEFORE this tool
// touches a single file.
//
// The elevation gate is about consent, not capability: nothing here needs
// root — it edits the user's own repo and their ~/.claude. Staying root
// through the writes would be a real hazard rather than a theoretical one,
// because every path the tool writes is one an agent could have replaced
// with a symlink beforehand (CLAUDE.md, .claude/, settings.local.json), and
// a root write follows that symlink to any target on the machine. Repairing
// ownership afterwards does not help: the damage is the write. Becoming the
// invoking user first reduces the whole class to "the user can write the
// user's own files", which was always true.
//
// setgroups/setgid must precede setuid — after setuid the process can no
// longer change its groups. The verification afterwards is the backstop: a
// drop that did not fully take effect must stop the tool, never continue.
//
// The environment is passed in rather than read here, so every branch that
// refuses is reachable from a test without being root.
func dropPrivileges(euid int, sudoUID, sudoGID, sudoUser string) (string, error) {
	mode, uid, gid, err := dropTarget(euid, sudoUID, sudoGID)
	if err != nil {
		return "", err
	}
	switch mode {
	case dropNotNeeded:
		return "running unelevated as uid " + strconv.Itoa(os.Getuid()), nil
	case dropImpossible:
		// Root with nobody to become. Continuing would write root-owned
		// files into a repo the agent session then cannot touch, through
		// paths this tool refuses to follow precisely because it is root.
		return "", fmt.Errorf("running as root with no SUDO_UID to return to, so there is no account to drop back to — run this with `sudo` from your own login instead of from a root shell")
	}
	groups, warning := invokingGroups(sudoUser, uid, gid)
	if err := syscall.Setgroups(groups); err != nil {
		return "", fmt.Errorf("cannot install the invoking user's groups %v: %v", groups, err)
	}
	if err := syscall.Setgid(gid); err != nil {
		return "", fmt.Errorf("cannot drop gid to %d: %v", gid, err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return "", fmt.Errorf("cannot drop uid to %d: %v", uid, err)
	}
	if os.Getuid() != uid || os.Geteuid() != uid || os.Getgid() != gid || os.Getegid() != gid {
		return "", fmt.Errorf("privilege drop did not take effect (uid %d/%d, gid %d/%d) — refusing to write anything as root",
			os.Getuid(), os.Geteuid(), os.Getgid(), os.Getegid())
	}
	msg := fmt.Sprintf("dropped root, now uid %d/gid %d with %d group(s) — nothing below is written with root's privileges", uid, gid, len(groups))
	if warning != "" {
		msg = warning + "\n" + msg
	}
	return msg, nil
}

// invokingHome resolves the home directory of the human behind sudo, NOT
// root's. `sudo` may or may not reset HOME depending on the machine's
// sudoers policy, so HOME is not trusted here: /task installed into
// /var/root/.claude/commands would be invisible to every real session.
func invokingHome() (string, error) {
	name := os.Getenv("SUDO_USER")
	if name == "" || name == "root" {
		return os.UserHomeDir()
	}
	if u, err := user.Lookup(name); err == nil && u.HomeDir != "" {
		return u.HomeDir, nil
	}
	if uid := os.Getenv("SUDO_UID"); uid != "" {
		if u, err := user.LookupId(uid); err == nil && u.HomeDir != "" {
			return u.HomeDir, nil
		}
	}
	return "", fmt.Errorf("cannot resolve the home directory of %q (the user behind sudo) — install the /task command by hand: copy commands/task.md to ~/.claude/commands/task.md", name)
}
