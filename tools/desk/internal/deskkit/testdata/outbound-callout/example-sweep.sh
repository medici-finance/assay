#!/bin/sh
# EXAMPLE house callout (invented values): block any write whose "text" fields contain a word
# listed in a file the deployment owns (one word per line, no blank lines). Documented in
# tools/desk/README.md. Reads the request on stdin; prints `allow` or `block <reason>`.
words="${EXAMPLE_WORDS_FILE:-/etc/example-house/withheld-words.txt}"
if [ ! -r "$words" ]; then
  echo "block the word list is unreadable"   # fail closed: no list, no verdict
  exit 0
fi
if grep -o '"text":"\([^"\\]\|\\.\)*"' | grep -qiF -f "$words"; then
  echo "block the write names a word on the house list"
else
  echo allow
fi
