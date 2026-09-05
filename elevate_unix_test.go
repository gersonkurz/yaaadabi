//go:build !windows

package main

import (
	"os"
	"os/user"
	"slices"
	"strings"
	"testing"
)

// dropTarget is the decision half of the privilege drop; the syscalls
// themselves need root, but which account to become — and when to refuse
// rather than keep root — is decided here and is testable.
func TestDropTargetDecides(t *testing.T) {
	cases := []struct {
		name         string
		euid         int
		uid, gid     string
		wantMode     dropMode
		wantU, wantG int
		wantErr      bool
	}{
		{"unelevated", 501, "", "", dropNotNeeded, -1, -1, false},
		{"unelevated with stale sudo vars", 501, "501", "20", dropNotNeeded, -1, -1, false},
		{"root via sudo", 0, "501", "20", dropToUser, 501, 20, false},
		{"root without sudo", 0, "", "", dropImpossible, -1, -1, false},
		{"root, non-numeric uid", 0, "root", "20", 0, -1, -1, true},
		{"root, non-numeric gid", 0, "501", "staff", 0, -1, -1, true},
		{"root, half-set", 0, "501", "", 0, -1, -1, true},
		{"root returning to root", 0, "0", "0", 0, -1, -1, true},
		{"root, negative uid", 0, "-1", "20", 0, -1, -1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode, uid, gid, err := dropTarget(tc.euid, tc.uid, tc.gid)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if mode != tc.wantMode || uid != tc.wantU || gid != tc.wantG {
				t.Fatalf("got (%v, %d, %d), want (%v, %d, %d)", mode, uid, gid, tc.wantMode, tc.wantU, tc.wantG)
			}
		})
	}
}

// The refusal branches are now reachable without root, because the
// environment is a parameter rather than something dropPrivileges reads.
func TestDropPrivilegesRefusesRatherThanKeepingRoot(t *testing.T) {
	// Root with nobody to return to: the tool must stop, not carry on and
	// write root-owned files into the human's repo.
	if _, err := dropPrivileges(0, "", "", ""); err == nil {
		t.Fatal("root without sudo must be refused, not warned about")
	} else if !strings.Contains(err.Error(), "sudo") {
		t.Errorf("the error should say how to run it properly: %v", err)
	}
	// Malformed or root-returning sudo variables must not be interpreted.
	for name, env := range map[string][2]string{
		"non-numeric uid": {"root", "20"},
		"half set":        {"501", ""},
		"returns to root": {"0", "0"},
	} {
		if _, err := dropPrivileges(0, env[0], env[1], "someone"); err == nil {
			t.Errorf("%s: expected a refusal, got none", name)
		}
	}
}

// Unelevated, dropPrivileges must be a no-op that still reports what it did.
func TestDropPrivilegesIsInertUnelevated(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}
	before := os.Getuid()
	msg, err := dropPrivileges(os.Geteuid(), os.Getenv("SUDO_UID"), os.Getenv("SUDO_GID"), os.Getenv("SUDO_USER"))
	if err != nil {
		t.Fatal(err)
	}
	if msg == "" {
		t.Error("no report from dropPrivileges")
	}
	if os.Getuid() != before {
		t.Fatalf("uid changed from %d to %d without root", before, os.Getuid())
	}
}

// Dropping to the primary group alone would lock the tool out of a repo the
// human can write through a supplementary group. The resolved list must
// therefore contain everything the human actually has — which this process,
// running AS the human, can simply compare against.
func TestInvokingGroupsKeepsSupplementaryGroups(t *testing.T) {
	me, err := user.Current()
	if err != nil {
		t.Skip("no current user to look up")
	}
	mine, err := os.Getgroups()
	if err != nil {
		t.Skip("cannot read this process's groups")
	}
	got, warning := invokingGroups(me.Username, os.Getuid(), os.Getgid())
	if warning != "" {
		t.Fatalf("resolution should have succeeded for the current user: %s", warning)
	}
	if len(mine) > 1 && len(got) <= 1 {
		t.Fatalf("dropped %d groups down to %v", len(mine), got)
	}
	for _, g := range mine {
		if !slices.Contains(got, g) {
			t.Errorf("group %d belongs to %s but was not carried over: %v", g, me.Username, got)
		}
	}
	if !slices.Contains(got, os.Getgid()) {
		t.Errorf("primary gid %d missing from %v", os.Getgid(), got)
	}
}

// An account that cannot be resolved must degrade to the primary group and
// SAY so, rather than failing the run or silently losing access.
func TestInvokingGroupsFallsBackLoudly(t *testing.T) {
	got, warning := invokingGroups("no-such-account-4f2c1a", 501, 20)
	if !slices.Equal(got, []int{20}) {
		t.Errorf("expected the primary group only, got %v", got)
	}
	if !strings.Contains(warning, "supplementary groups") {
		t.Errorf("the fallback must be reported: %q", warning)
	}
}

// The home directory must be the human's, not root's: /task installed under
// /var/root would be invisible to every real session.
func TestInvokingHomeFollowsSudoUser(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SUDO_USER", "")
	t.Setenv("HOME", home)
	got, err := invokingHome()
	if err != nil || got != home {
		t.Fatalf("without sudo, expected $HOME %q, got %q (%v)", home, got, err)
	}

	// With SUDO_USER set, the lookup — not $HOME — decides.
	me, err := user.Current()
	if err != nil {
		t.Skip("no current user to look up")
	}
	t.Setenv("SUDO_USER", me.Username)
	t.Setenv("HOME", "/var/root")
	got, err = invokingHome()
	if err != nil {
		t.Fatal(err)
	}
	if got != me.HomeDir {
		t.Fatalf("expected %s's home %q, got %q", me.Username, me.HomeDir, got)
	}
}
