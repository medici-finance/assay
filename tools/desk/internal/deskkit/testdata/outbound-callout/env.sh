#!/bin/sh
# Invented stub: dumps its environment beside itself and allows.
echo run >> "${0%/*}/ran.log"
cat > /dev/null
env > "${0%/*}/env.dump"
echo allow
