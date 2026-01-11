#!/bin/bash
set -e

# Get the root directory of the repository
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

echo "Building mstl..."
go build -o "$ROOT_DIR/bin/mstl" "$ROOT_DIR/cmd/mstl"
echo "mstl built at bin/mstl"

echo "Building mstl-gh..."
go build -o "$ROOT_DIR/bin/mstl-gh" "$ROOT_DIR/cmd/mstl-gh"
echo "mstl-gh built at bin/mstl-gh"
