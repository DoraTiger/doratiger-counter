#!/usr/bin/env bash

set -euo pipefail

name=${1:?binary name is required}
build_dir=${2:?build directory is required}
release_dir=${3:?release directory is required}

mkdir -p "$release_dir"
checksums="$release_dir/checksums.txt"
: > "$checksums"

for target_dir in "$build_dir"/linux-*; do
    target=${target_dir##*/}
    archive="$release_dir/$name-$target.tar.gz"
    tar -C "$target_dir" -czf "$archive" "$name"
    sha256sum "$archive" | sed "s|  .*|  ${archive##*/}|" >> "$checksums"
done

sort -o "$checksums" "$checksums"
