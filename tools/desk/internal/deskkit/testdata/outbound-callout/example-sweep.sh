#!/bin/sh
# EXAMPLE house callout (invented values): block any write whose "text" fields contain a word
# or phrase listed in a file the deployment owns. Reads the request on stdin; prints `allow`
# or `block <reason>`. List: one word or phrase per line, one space between a phrase's
# words, no blank lines, LF line endings; matched case-insensitively as fixed strings.
# It answers `allow` ONLY when every step below ran and the match reported "no match": a tool
# that is missing, fails or is killed is a block, as an unreadable list is.
# Its own tools, never the caller's: the PATH value it is handed is the calling process's.
PATH=/usr/bin:/bin
export PATH
words="${0%/*}/words.txt"
# A blank line would match every write and a CRLF ending would match none: refuse either.
grep -q -e "$(printf '\r')" -e '^$' "$words"
case $? in
  1) ;;
  0) echo "block the word list has a blank line or a CRLF line ending"; exit 0 ;;
  *) echo "block the word list is unreadable"; exit 0 ;;   # fail closed: no list, no verdict
esac
# Each "text" value is a JSON string: decode it before matching. `\\` is held aside first so
# an escaped backslash followed by n is not read as a line break; every whitespace escape and
# \uXXXX (control and line-separator characters) becomes a space; any other escape (`\"`,
# `\/`) is the character itself; runs of spaces fold to one, so a listed phrase matches
# across a line break, a tab or a run of spaces in the write. Each step's own exit status is
# checked: a pipeline reports only its last command's.
texts=$(grep -oE '"text":"([^"\\]|\\.)*"')
case $? in 0 | 1) ;; *) echo "block the text could not be extracted"; exit 0 ;; esac
hold=$(printf '\001')
decoded=$(printf '%s\n' "$texts" |
  sed -e 's/^"text":"//' -e 's/"$//' \
      -e 's/\\\\/'"$hold"'/g' \
      -e 's/\\u[0-9A-Fa-f]\{4\}/ /g' -e 's/\\[bfnrt]/ /g' -e 's/\\\(.\)/\1/g' \
      -e 's/'"$hold"'/\\/g' -e 's/  */ /g') || { echo "block the text could not be decoded"; exit 0; }
printf '%s\n' "$decoded" | grep -qiF -f "$words"
case $? in
  0) echo "block the write names a word on the house list" ;;
  1) echo allow ;;
  *) echo "block the word match did not run" ;;
esac
