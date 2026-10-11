#!/bin/sh
# Invented stub: records that it ran and what it was asked, answers allow.
echo run >> "${0%/*}/ran.log"
cat > "${0%/*}/request.json"
echo allow
