#!/usr/bin/env python3
"""Run bounded readiness mutations after committing, using the shared runner."""
import importlib.util
from pathlib import Path

path = Path(__file__).with_name("01-mutate.py")
spec = importlib.util.spec_from_file_location("milestone_mutations", path)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)
runner.main(Path(__file__).with_name("07-mutations.json"), "milestone-mutations-v1")
