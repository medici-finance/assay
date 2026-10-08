#!/bin/sh
# Invented stub: records that it ran, answers block with an invented rule name.
echo run >> "${0%/*}/ran.log"
cat > /dev/null
echo "block example-house-rule"
echo "example-house-diagnostic" >&2
