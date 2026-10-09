#!/bin/sh
# Invented stub: exits 0 and prints nothing.
echo run >> "${0%/*}/ran.log"
cat > /dev/null
