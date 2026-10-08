#!/bin/sh
# EXAMPLE house callout (invented values): block any write whose "text" fields contain a word
# or phrase listed in a file the deployment owns. Reads the request on stdin; prints `allow`
# or `block <reason>`. List: one word or phrase per line, one space between a phrase's
# words, no blank lines; matched case-insensitively as fixed strings.
words="${0%/*}/words.txt"
if [ ! -r "$words" ]; then
  echo "block the word list is unreadable"   # fail closed: no list, no verdict
  exit 0
fi
# Each "text" value is a JSON string: decode it before matching. `\\` is held aside first so
# an escaped backslash followed by n is not read as a line break; every whitespace escape and
# \uXXXX (control and line-separator characters) becomes a space; any other escape (`\"`,
# `\/`) is the character itself; runs of spaces fold to one, so a listed phrase matches
# across a line break, a tab or a run of spaces in the write.
hold=$(printf '\001')
if grep -oE '"text":"([^"\\]|\\.)*"' |
  sed -e 's/^"text":"//' -e 's/"$//' \
      -e 's/\\\\/'"$hold"'/g' \
      -e 's/\\u[0-9A-Fa-f]\{4\}/ /g' -e 's/\\[bfnrt]/ /g' -e 's/\\\(.\)/\1/g' \
      -e 's/'"$hold"'/\\/g' -e 's/  */ /g' |
  grep -qiF -f "$words"; then
  echo "block the write names a word on the house list"
else
  echo allow
fi
