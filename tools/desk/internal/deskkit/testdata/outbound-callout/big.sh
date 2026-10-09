#!/bin/sh
# Invented stub: prints 100 KiB on one line, then allow.
echo run >> "${0%/*}/ran.log"
cat > /dev/null
head -c 102400 /dev/zero | tr "\000" "a"
echo
echo allow
