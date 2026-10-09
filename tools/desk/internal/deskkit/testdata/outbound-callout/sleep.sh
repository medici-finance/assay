#!/bin/sh
# Invented stub: never answers within any permitted deadline.
echo run >> "${0%/*}/ran.log"
sleep 30
echo allow
