#!/usr/bin/env bash
#
# Copyright (c) 2026 Karagatan LLC.
# SPDX-License-Identifier: BUSL-1.1
#
# Coordinated release for the go.arpabet.com/servion repo.
#
# Unlike a pure multi-module monorepo, servion has a ROOT module
# (go.arpabet.com/servion) plus optional submodules (e.g. go.arpabet.com/servion/grpc).
# The root module is tagged "vX.Y.Z"; each submodule is tagged "<subdir>/vX.Y.Z"
# (e.g. grpc/v0.3.0), per the Go multi-module convention.
#
# One shared version moves every module; an interface change ripples into all of
# them. A module carrying an extra change takes a higher patch via a per-module
# override, keyed by subdir ("." for the root module):
#
#     ./release.sh v0.3.0 grpc=v0.3.1
#
# Modules are discovered automatically (every dir with a go.mod, excluding
# examples). Before tagging, internal `require go.arpabet.com/servion[/X]` lines
# are pinned to the release version and the local-dev
# `replace go.arpabet.com/servion[/X] => ..` bootstrap directives are stripped
# (consumers ignore replaces anyway; this keeps published go.mods clean).
# `go.work` covers local dev post-release.
#
# Tag a clean build first: `make test` before releasing.
#
# Usage: ./release.sh [--dry-run] [--no-push] <version> [module=version ...]
#
# Compatible with the bash 3.2 that ships on macOS (no associative arrays/mapfile).
#
set -euo pipefail

PREFIX="go.arpabet.com/servion"
REMOTE="origin"
DRY_RUN=0; NO_PUSH=0
VERSION=""; OVERRIDES=""

die() { echo "error: $*" >&2; exit 1; }
semver_ok() { case "$1" in v[0-9]*.[0-9]*.[0-9]*) return 0;; *) return 1;; esac; }

for a in "$@"; do
	case "$a" in
		--dry-run) DRY_RUN=1 ;;
		--no-push) NO_PUSH=1 ;;
		*=v*)      OVERRIDES="$OVERRIDES $a" ;;
		v*)        VERSION="$a" ;;
		*)         die "unrecognized arg: $a" ;;
	esac
done
[ -n "$VERSION" ] || die "usage: ./release.sh [--dry-run] [--no-push] <version> [module=version ...]"
semver_ok "$VERSION" || die "'$VERSION' is not vMAJOR.MINOR.PATCH"

# release version for a module key (subdir, or "." for the root module)
ver_for() {
	local tok
	for tok in $OVERRIDES; do
		case "$tok" in "$1="*) echo "${tok#*=}"; return;; esac
	done
	echo "$VERSION"
}

# go import path of a module key
mod_path() {
	case "$1" in
		.) echo "$PREFIX" ;;
		*) echo "$PREFIX/$1" ;;
	esac
}

# git tag name of a module key at a version
tag_for() {
	case "$1" in
		.) echo "$2" ;;
		*) echo "$1/$2" ;;
	esac
}

[ -z "$(git status --porcelain)" ] || die "working tree is dirty; commit or stash first"

# discover module keys: "." for the root module, "<subdir>" for each submodule.
# examples are never their own modules and are skipped if they ever gain a go.mod.
MODULES="$(find . -name go.mod -not -path './.*' -not -path '*/examples/*' \
	| sed 's#/go.mod$##; s#^\./##' | sort)"
[ -n "$MODULES" ] || die "no modules found"

echo "Release plan (shared $VERSION):"
for m in $MODULES; do
	printf "  %-12s -> tag %s\n" "$m" "$(tag_for "$m" "$(ver_for "$m")")"
done

# rewrite each go.mod: strip bootstrap replaces, pin internal requires
for m in $MODULES; do
	gm="$m/go.mod"
	# strip local-dev replaces of any internal module (root or submodule)
	perl -i -ne "print unless m{^replace \Q$PREFIX\E(/|\s)}" "$gm"
	# pin internal require versions to the release
	for dep in $MODULES; do
		dpath="$(mod_path "$dep")"
		dv="$(ver_for "$dep")"
		perl -i -pe "s{(\Q$dpath\E)\s+v\S+}{\$1 $dv}g" "$gm"
	done
done

if [ "$DRY_RUN" -eq 1 ]; then
	echo "--- dry run: go.mod changes below, nothing committed ---"
	git --no-pager diff -- '*go.mod'
	git checkout -- . 2>/dev/null || true
	exit 0
fi

git add -A
git commit -m "release $VERSION"
TAGS=""
for m in $MODULES; do
	t="$(tag_for "$m" "$(ver_for "$m")")"
	git tag "$t"
	TAGS="$TAGS $t"
done
echo "tagged:$TAGS"

if [ "$NO_PUSH" -eq 1 ]; then
	echo "--no-push: created commit + tags locally; not pushed"
	exit 0
fi
# root module is tagged first (MODULES is sorted, "." precedes submodules) so the
# submodules' `require go.arpabet.com/servion <version>` resolves once published.
git push "$REMOTE" HEAD
git push "$REMOTE" $TAGS
echo "released $VERSION"
