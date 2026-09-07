# justfile — build and check the wiring tool, on macOS/Linux and on Windows.
#
# Two things here are deliberate and worth knowing before editing:
#
# `build` passes neither -o nor an out/ directory. `go build` already names the
# binary yaaadabi / yaaadabi.exe per platform, and it must land NEXT TO
# protocol.md: the binary's own directory is the default loop directory, which
# is what makes wiring a repo work with no flags (README, "A different protocol
# entirely").
#
# `clean` delegates the deletion to `go clean .` rather than spelling out
# `rm -f` and `del /Q` per platform: the toolchain removes `DIR` and
# `DIR.exe` (`go help clean` lists it as `DIR(.exe) from go build`), so both
# names go on either host and nothing hand-written is left for a Windows run
# to have to prove. The package argument is explicit because that is the
# condition `go help clean` states for removing these files at all.
#
# Know what that recipe is, though: `go clean` selects by NAME, not by
# provenance. It also deletes `test.out`, `build.out`, `*.[568ao]`, `*.so`,
# `_obj/`, `_test/`, `_testmain.go` and `DIR.test(.exe)` when present, no
# matter who created them. Nothing in this repo carries those names — that is
# what makes the recipe safe HERE, not a general guarantee.
#
# `vet` checks the host build AND both build-tagged halves. Half of
# isElevated/dropPrivileges/invokingHome is excluded by whichever platform you
# are on, and the excluded half breaks silently — so both are named explicitly
# instead of assuming the host is a Mac. On Windows one of the two repeats the
# host vet; that costs a second and keeps the recipe host-independent.

set windows-shell := ["cmd.exe", "/c"]

# Show available commands
default:
    @just --list

# Build the wiring tool next to the prose files
build:
    @echo Building...
    go build

# Remove the built binary
clean:
    @echo Cleaning...
    go clean .

# Clean then build
rebuild: clean build

# Vet the host build and both build-tagged halves
vet:
    @echo Vetting host build...
    go vet ./...
    @echo Vetting the Windows half...
    {{ if os() == "windows" { "set GOOS=windows&& go vet ./..." } else { "GOOS=windows go vet ./..." } }}
    @echo Vetting the unix half...
    {{ if os() == "windows" { "set GOOS=darwin&& go vet ./..." } else { "GOOS=darwin go vet ./..." } }}

# Run the tests, never from the cache (-count=1: a replayed green is not evidence)
test:
    @echo Testing...
    go test -count=1 ./...

# Everything the Review loop's Verify parameter requires
verify: vet test
    @echo Verify complete.
