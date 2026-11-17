#!/usr/bin/env bash
# Builds the release documents (see .specs/netcheck-restructure.md) into
# build/docs/ (not dist/, which goreleaser cleans), with pinned container images: Mermaid diagrams to PDF with
# mermaid-cli, then each document to PDF with pandoc, LaTeX and Eisvogel.
#
#   docs/build.sh                  # every document
#   docs/build.sh guide            # one document
#
# DOCS_VERSION sets the version printed in the documents (default: git
# describe); DOCS_ENGINE the container engine (default: docker, else podman).
set -euo pipefail

readonly PANDOC_IMAGE="docker.io/pandoc/extra:3.11.0.0-debian@sha256:fa8ccae75418449568f10f1842706aee1885ab24fba54f5aec08516260fc0166"
readonly MERMAID_IMAGE="ghcr.io/mermaid-js/mermaid-cli/mermaid-cli:12.0.0@sha256:fa995339034aae7e5cd4f61482248b7f5c51be355b1a6f6eda11a2bbf8401f5f"
readonly DOCUMENTS=(guide)

declare -A FILE_NAMES=(
	[guide]=guide
)

die() {
	echo "docs: $*" >&2
	exit 1
}

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo"

engine="${DOCS_ENGINE:-}"
if [[ -z "$engine" ]]; then
	if command -v docker >/dev/null; then engine=docker; elif command -v podman >/dev/null; then engine=podman; else die "neither docker nor podman is installed"; fi
fi
version="${DOCS_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
version="${version#v}"
date="$(date -u +%Y-%m-%d)"

documents=("$@")
[[ ${#documents[@]} -gt 0 ]] || documents=("${DOCUMENTS[@]}")
for d in "${documents[@]}"; do
	[[ -n "${FILE_NAMES[$d]:-}" ]] || die "unknown document $d (one of: ${DOCUMENTS[*]})"
done

out=build/docs work=build/docs/work
rm -rf "$out"
mkdir -p "$work/diagrams"

# run IMAGE ARGS...: runs a container as the current user, with the
# repository mounted as its working directory
run() {
	local image="$1"
	shift
	# HOME: fontconfig and LaTeX need a writable cache for this user
	"$engine" run --rm --user "$(id -u):$(id -g)" --env HOME=/tmp --env LANG=C.UTF-8 --volume "$repo:/data" --workdir /data "$image" "$@"
}

# diagrams: each docs/common/diagrams/NAME.md holds one mermaid block
for source in docs/common/diagrams/*.md; do
	name="$(basename "$source" .md)"
	awk '/^```mermaid$/ { inside = 1; next } /^```$/ && inside { exit } inside' "$source" >"$work/diagrams/$name.mmd"
	[[ -s "$work/diagrams/$name.mmd" ]] || die "$source has no mermaid block"
	echo "docs: diagram $name"
	run "$MERMAID_IMAGE" --quiet --configFile /data/docs/common/mermaid.json \
		--input "/data/$work/diagrams/$name.mmd" --output "/data/$work/diagrams/$name.pdf" >/dev/null
done

# the front matter, with the version and date
sed -e "s/\\\$version\\\$/$version/g" -e "s/\\\$date\\\$/$date/g" docs/common/front-matter.md >"$work/front-matter.md"

for d in "${documents[@]}"; do
	pdf="$out/netcheck-${FILE_NAMES[$d]}-$version.pdf"
	echo "docs: $pdf"
	# shellcheck disable=SC2046 # the chapters, in file order
	run "$PANDOC_IMAGE" \
		--defaults docs/common/defaults.yaml \
		--metadata-file "docs/$d/metadata.yaml" \
		--metadata date="Version $version · $date" \
		--variable header-right="Version $version" \
		--output "$pdf" \
		"$work/front-matter.md" $(ls docs/"$d"/[0-9][0-9]-*.md)
done
