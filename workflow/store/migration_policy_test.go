package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPGFoundationDirectivePolicy(t *testing.T) {
	// Goose v3.29.0 recognizes annotation comments without a separating space,
	// removes every double dash, and matches commands case-insensitively.
	for _, directive := range []string{
		"--+goose NO TRANSACTION", "-- -- +goose NO TRANSACTION",
		"--+goose no transaction", "--+goose ENVSUB ON",
		"-- -- +goose envsub off", "--+goose Down",
		"-- -- +goose down", "--+goose Up", "--+goose unknown",
	} {
		t.Run(directive, func(t *testing.T) {
			source := extraSource(t, directive+"\nSELECT 1;")
			if _, err := loadManifest(source); err == nil {
				t.Fatal("unsafe or duplicate annotation admitted")
			}
			// Invalid configuration independently proves refusal precedes any
			// connection attempt, including DDL that could otherwise auto-commit.
			var got *Error
			if err := migrate(context.Background(), Config{}, source); !errors.As(err, &got) || got.Code != InvalidManifest {
				t.Fatalf("expected pre-connection manifest refusal, got %v", err)
			}
		})
	}
	t.Run("normalized-forward-migration", func(t *testing.T) {
		_, c, db := fixture(t)
		source := extraSource(t, "-- -- +goose statementbegin\nCREATE TABLE public.directive_canary(id integer);\n--+goose statementend")
		for name, file := range source {
			if strings.HasSuffix(name, "_control.sql") {
				source[name] = &fstest.MapFile{Data: []byte(strings.Replace(string(file.Data), "-- +goose Up", "--+goose uP", 1))}
			}
		}
		if err := migrate(context.Background(), c, source); err != nil {
			t.Fatal(err)
		}
		var present bool
		if err := db.QueryRow("SELECT to_regclass('public.directive_canary') IS NOT NULL").Scan(&present); err != nil || !present {
			t.Fatalf("valid normalized Goose migration did not run: present=%v err=%v", present, err)
		}
	})
}
