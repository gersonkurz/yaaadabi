//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

const elevationHint = "run this from an elevated (administrator) shell — elevation is the out-of-process human consent this tool requires"

// isElevated reports whether the process holds an elevated token, queried
// in-process from the Windows API — nothing PATH- or env-resolvable to
// spoof (a subprocess check was reviewer-bypassed with a fake net.exe).
func isElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// dropPrivileges has no Windows counterpart worth faking: an elevated
// process is the SAME user with a fuller token, so there is no account to
// return to, and dropping the token mid-process is not supported. The
// symlink hazard that motivates the unix drop is also much narrower here —
// creating a symlink needs privilege or Developer Mode. Windows behaviour
// is therefore unchanged from before this file existed.
func dropPrivileges(euid int, sudoUID, sudoGID, sudoUser string) (string, error) {
	return "elevated as the same user — Windows has no account to drop back to", nil
}

// invokingHome is the process's own home — elevating does not change user.
func invokingHome() (string, error) { return os.UserHomeDir() }
