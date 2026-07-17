#!/bin/sh
set -eu

seal=${1:?StateSeal binary path is required}

cp fixtures/fixed.go.txt counter.go
"$seal" submit

# Regress the terminal state after a verified checkpoint was preserved.
cp fixtures/regressed.go.txt counter.go
