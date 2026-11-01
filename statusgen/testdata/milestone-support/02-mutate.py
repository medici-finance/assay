#!/usr/bin/env python3
"""Use the existing bounded offline runner for the reader contract."""
import importlib.util
from pathlib import Path

path = Path(__file__).with_name("01-mutate.py")
spec = importlib.util.spec_from_file_location("milestone_mutations", path)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)
runner.main(Path(__file__).with_name("02-mutations.json"), "readercaps-mutations-v1")
