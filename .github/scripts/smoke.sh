#!/usr/bin/env bash
#
# Smoke test: prove the built binary actually starts and answers a real query.
#
# A job that only compiles cannot catch a package-level init() panic — that is how
# gnoverse/gnopls#59 shipped broken for 2.5 months with CI green. Every invocation,
# `version` included, died before main() ran.
#
# Usage: .github/scripts/smoke.sh [path-to-gnopls]

set -euo pipefail

GNOPLS="${1:-gnopls}"

ws="$(mktemp -d)"
trap 'rm -rf "$ws"' EXIT

: > "$ws/gnowork.toml"
cat > "$ws/gnomod.toml" <<'EOF'
module = "gno.land/p/demo/smoke"
gno = "0.9"
EOF
cat > "$ws/hello.gno" <<'EOF'
package smoke

func Hello() string {
	return "hi"
}
EOF

echo "== gnopls version =="
"$GNOPLS" version

echo "== gnopls symbols =="
symbols="$(cd "$ws" && "$GNOPLS" symbols hello.gno)"
echo "$symbols"
if [[ "$symbols" != *"Hello Function"* ]]; then
	echo "smoke: FAIL — expected 'Hello Function' in symbols output" >&2
	exit 1
fi

echo "== gnopls definition =="
definition="$(cd "$ws" && "$GNOPLS" definition hello.gno:3:6)"
echo "$definition"
if [[ "$definition" != *"func Hello() string"* ]]; then
	echo "smoke: FAIL — expected 'func Hello() string' in definition output" >&2
	exit 1
fi

echo "smoke: ok"
