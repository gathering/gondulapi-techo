#!/bin/bash

set -eu

OUT_DIR="backup"

if [[ ! -d $OUT_DIR ]]; then
        echo "Output dir doesn't exist: $OUT_DIR" >&2
        exit 1
fi

timestamp="$(date --iso-8601=seconds)"
out_file="${OUT_DIR}/db-backup-${timestamp}.gz"

manage/db-dump.sh | gzip > "$out_file"

echo "Created backup $out_file."
