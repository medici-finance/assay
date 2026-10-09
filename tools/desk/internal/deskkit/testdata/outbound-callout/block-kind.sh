#!/bin/sh
# Invented stub: blocks ONLY a write of the kind named in the file `kind` beside itself and
# allows every other kind, appending the kind of every request it is asked to `asked.log`.
# A test uses it to prove one kind of write was put to the callout: a stub that blocked
# unconditionally would refuse the first write it saw and say nothing about the rest.
d="${0%/*}"
echo run >> "$d/ran.log"
req=$(cat)
want=$(cat "$d/kind")
printf '%s\n' "$req" | sed -n 's/.*"kind":"\([a-z]*\)".*/\1/p' >> "$d/asked.log"
case "$req" in
  *"\"kind\":\"$want\""*) echo "block example-house-rule" ;;
  *) echo allow ;;
esac
