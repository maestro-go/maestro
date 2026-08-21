#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="${OUT_DIR:-$ROOT_DIR/.badges}"

VERSION="${VERSION:-unknown}"
GO_VERSION="${GO_VERSION:-1.25+}"
LINT="${LINT:-passing}"
COVERAGE="${COVERAGE:-0}"
LICENSE="${LICENSE:-MIT}"

# Pick a color based on the coverage percentage (matching the >= 70% CI gate).
coverage_color() {
  local c="$1"
  if awk "BEGIN { exit !($c >= 70) }"; then
    echo "#28a745" # green
  elif awk "BEGIN { exit !($c >= 50) }"; then
    echo "#dbab09" # yellow
  else
    echo "#d73a49" # red
  fi
}

LINT_COLOR="#28a745"
if [ "$LINT" != "passing" ]; then
  LINT_COLOR="#d73a49"
fi

# badge <label> <value> <color> <outfile>
badge() {
  local label="$1" value="$2" color="$3" outfile="$4"

  local label_w=$(( ${#label} * 7 + 14 ))
  local value_w=$(( ${#value} * 7 + 14 ))
  local total=$(( label_w + value_w ))

  mkdir -p "$(dirname "$outfile")"
  cat > "$outfile" <<EOF
<svg xmlns="http://www.w3.org/2000/svg" width="$total" height="20" role="img" aria-label="$label: $value">
  <title>$label: $value</title>
  <linearGradient id="s" x2="0" y2="100%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="r">
    <rect width="$total" height="20" rx="3" fill="#fff"/>
  </clipPath>
  <g clip-path="url(#r)">
    <rect width="$label_w" height="20" fill="#555"/>
    <rect x="$label_w" width="$value_w" height="20" fill="$color"/>
    <rect width="$total" height="20" fill="url(#s)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11">
    <text x="$(( label_w / 2 ))" y="15" fill="#010101" fill-opacity=".3">$label</text>
    <text x="$(( label_w / 2 ))" y="14">$label</text>
    <text x="$(( label_w + value_w / 2 ))" y="15" fill="#010101" fill-opacity=".3">$value</text>
    <text x="$(( label_w + value_w / 2 ))" y="14">$value</text>
  </g>
</svg>
EOF
}

COV_COLOR="$(coverage_color "$COVERAGE")"

# Generate the stable "release" set referenced by the README, plus a
# per-version copy so every release keeps its own badges.
for target_dir in "$OUT_DIR/release" "$OUT_DIR/$VERSION"; do
  badge "release" "$VERSION" "#0969da" "$target_dir/version.svg"
  badge "go" "$GO_VERSION" "#00ADD8" "$target_dir/go.svg"
  badge "golangci-lint" "$LINT" "$LINT_COLOR" "$target_dir/lint.svg"
  badge "coverage" "$COVERAGE%" "$COV_COLOR" "$target_dir/coverage.svg"
  badge "license" "$LICENSE" "#7D929E" "$target_dir/license.svg"
done

echo "Badges generated for $VERSION (coverage $COVERAGE%, lint $LINT) in $OUT_DIR"