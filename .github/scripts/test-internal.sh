#!/usr/bin/env bash
#
# Runs ./internal/... minus the packages listed in .github/known-broken-tests.txt.
#
# Two things make this suite runnable at all:
#
#  1. GOPACKAGESDRIVER=off. With no driver configured, findExternalDriver falls back to
#     os.Executable() plus a "resolve" argument — inside a test binary that is
#     `<pkg>.test resolve`, i.e. the test binary re-executing itself, which is why so many
#     of these packages used to hang until the test timeout rather than fail.
#  2. The exclusion list, so the 77 packages that do pass can be a merge gate today
#     instead of waiting for the other 22 to be triaged.
#
# The list is measured on linux (what CI runs); macOS diverges on a couple of packages.

set -euo pipefail

cd "$(dirname "$0")/../.."

export GOPACKAGESDRIVER=off

list=".github/known-broken-tests.txt"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# strip comments and trailing "# ..." annotations, prefix with the module path
sed -e 's/#.*//' -e 's/[[:space:]]*$//' -e '/^$/d' "$list" \
	| sed 's|^|github.com/gnoverse/gnopls/|' \
	| sort -u > "$tmp/excluded"

# A stale entry would silently stop excluding anything — fail instead of rotting.
if ! go list -e "$(tr '\n' ' ' < "$tmp/excluded")" > /dev/null 2>&1; then
	while read -r pkg; do
		if ! go list "$pkg" > /dev/null 2>&1; then
			echo "known-broken-tests.txt names a package that no longer exists: $pkg" >&2
			exit 1
		fi
	done < "$tmp/excluded"
fi

go list ./internal/... | grep -vxFf "$tmp/excluded" > "$tmp/pkgs"

echo "running $(wc -l < "$tmp/pkgs") packages, excluding $(wc -l < "$tmp/excluded")"
xargs go test -count=1 -timeout 15m < "$tmp/pkgs"
