#!/bin/sh
# Compatibility for existing Verify rows; stock rules, no label exceptions.
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
IMAGE_SCAN_INTEGRATION=1 exec python3 "$HERE/image_secret_scan_test.py" Engine.test_clean_label_strings_need_no_exceptions Engine.test_same_file_multiple_keys_and_inline_ignore
