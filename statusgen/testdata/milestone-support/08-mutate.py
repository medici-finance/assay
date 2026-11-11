#!/usr/bin/env python3
"""Reuse the existing bounded, offline, restore-on-exit mutation runner."""
import importlib.util
from pathlib import Path

path = Path(__file__).with_name("01-mutate.py")
spec = importlib.util.spec_from_file_location("milestone_mutations", path)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)
runner.main(Path(__file__).with_name("08-mutations.json"), "brief-identity-mutations-v1")
