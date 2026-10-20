package main

import "github.com/medici-finance/assay/tools/desk/internal/reviewflow"

func humanGateDeclared(title, body string, labels []string) (bool, string) {
	return reviewflow.HumanGateDeclared(title, body, labels)
}
