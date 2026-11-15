#!/usr/bin/env python3
"""Delegate compatibility preparation arguments to the shared cellctl CLI."""
import os
import sys

os.execv('/opt/assay-preparation/cellctl',
         ['cellctl', 'compat-prepare', *sys.argv[1:]])
