#!/usr/bin/env python3
"""Use the existing bounded offline runner for the negotiated brief-v4 schema."""
import importlib.util
from pathlib import Path

path = Path(__file__).with_name("01-mutate.py")
spec = importlib.util.spec_from_file_location("milestone_mutations", path)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)
runner.main(Path(__file__).with_name("04-mutations.json"), "brief-v4-mutations-v1")
