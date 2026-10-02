package main

import "github.com/medici-finance/assay/tools/desk/internal/tick"

const grammarVersion = tick.Version

var Grammar = tick.Grammar

type ValidationError = tick.ValidationError

var Validate = tick.Validate
var Check = tick.Check

var errEmptyInput = tick.ErrEmptyInput
