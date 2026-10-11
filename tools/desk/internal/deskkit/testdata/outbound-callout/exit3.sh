#!/bin/sh
# Invented stub: exits non-zero.
echo run >> "${0%/*}/ran.log"
cat > /dev/null
exit 3
