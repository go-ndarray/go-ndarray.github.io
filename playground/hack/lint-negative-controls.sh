#!/usr/bin/env bash
# Copyright (c) the go-ndarray authors.
# SPDX-License-Identifier: BSD-3-Clause
#
# Negative controls for the two UI guards. A green `go vet -vettool=…` only
# proves a guard is SILENT on this tree; this script proves each one BITES on
# this app's own code:
#
#   bricolint  a raw painter draw call spliced into State.Draw   -> must fail
#   mvvmlint   a direct write of a widget's state field in app.go -> must fail
#
# and that both are green again once the injection is removed.
#
# Usage: BRICOLINT=/path/bricolint MVVMLINT=/path/mvvmlint bash hack/lint-negative-controls.sh
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
target="$root/app.go"
backup="$(mktemp)"
cp "$target" "$backup"
restore() { [ -s "$backup" ] && cp "$backup" "$target"; rm -f "$backup"; }
trap restore EXIT

vet() { ( cd "$root" && GOWORK=off go vet -vettool="$1" ./... ) >/dev/null 2>&1 && echo 0 || echo 1; }

# inject <anchor> <line>: put line right after the anchor line in app.go.
inject() {
  grep -qF "$1" "$backup" || { echo "anchor not found: $1" >&2; exit 2; }
  awk -v a="$1" -v l="$2" '{ print } index($0, a) { print l }' "$backup" > "$target"
}

for guard in bricolint mvvmlint; do
  case "$guard" in
    bricolint) tool="${BRICOLINT:?set BRICOLINT}"
               anchor='p := painter.NewPixelPainter(buf, s.w, s.h)'
               line='	p.FillRect(painter.Rect{}, painter.RGBA{}) // negative control' ;;
    mvvmlint)  tool="${MVVMLINT:?set MVVMLINT}"
               anchor='s.steps = toolkit.NewListBox(nil)'
               line='	s.steps.Items = nil // negative control' ;;
  esac
  cp "$backup" "$target"
  [ "$(vet "$tool")" = 0 ] || { echo "FAIL: $guard is red on the clean tree"; exit 1; }
  inject "$anchor" "$line"
  [ "$(vet "$tool")" = 1 ] || { echo "FAIL: $guard stayed green with an injected violation"; exit 1; }
  cp "$backup" "$target"
  [ "$(vet "$tool")" = 0 ] || { echo "FAIL: $guard did not return to green"; exit 1; }
  echo "ok: $guard bites on an injected violation and is silent otherwise"
done
