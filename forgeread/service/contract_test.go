package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/service"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "service", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func literalRequest(t *testing.T) service.Request {
	t.Helper()
	q, err := service.DecodeRequest(fixture(t, "request-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	return q
}
func literalResponse(t *testing.T, q service.Request) service.Response {
	t.Helper()
	r, err := service.DecodeResponse(fixture(t, "response-source.json"), q)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func sameJSON(t *testing.T, a, b []byte) {
	t.Helper()
	var x, y any
	dx, dy := json.NewDecoder(bytes.NewReader(a)), json.NewDecoder(bytes.NewReader(b))
	dx.UseNumber()
	dy.UseNumber()
	if dx.Decode(&x) != nil || dy.Decode(&y) != nil || !reflect.DeepEqual(x, y) {
		t.Fatal("literal wire value changed")
	}
}

// regression: class service-wire
func TestForgeServiceContractCompatibility(t *testing.T) {
	q := literalRequest(t)
	if q.Operation != service.FileAt || q.Repository.Provider != mirror.GitHub || q.Repository.Host != "github.example" || q.Repository.Path != "example/project" || q.Source.Revision != strings.Repeat("a", 40) || q.Source.Path != "readme.txt" {
		t.Fatal("literal request identity/source changed")
	}
	b, err := service.EncodeRequest(q)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, b, fixture(t, "request-source.json"))
	r := literalResponse(t, q)
	if string(r.Source.Content) != "fixture\n" || r.Metadata.State != service.Confirmed || r.Metadata.Complete == nil || !*r.Metadata.Complete || r.Scope.AccessEpoch != "epoch-1" {
		t.Fatal("literal result/provenance changed")
	}
	b, err = service.EncodeResponse(r, q)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, b, fixture(t, "response-source.json"))
	t.Run("closed", func(t *testing.T) {
		raw := string(fixture(t, "request-source.json"))
		for name, bad := range map[string]string{
			"schema":    strings.Replace(raw, "forge-service-v1", "forge-service-v9", 1),
			"operation": strings.Replace(raw, "file-at", "arbitrary-command", 1),
			"role":      strings.Replace(raw, `"operation":`, `"role":"admin","operation":`, 1),
			"url":       strings.Replace(raw, `"operation":`, `"url":"https://other.example/","operation":`, 1),
			"duplicate": strings.Replace(raw, `"id":`, `"id":"other","id":`, 1),
			"case":      strings.Replace(raw, `"schema":`, `"Schema":`, 1),
			"trailing":  raw + "{}",
			"bound":     strings.Repeat(" ", service.MaxMessageBytes) + raw,
		} {
			t.Run(name, func(t *testing.T) {
				if _, err := service.DecodeRequest([]byte(bad)); err == nil {
					t.Fatal("invalid request accepted")
				}
			})
		}
	})
	t.Run("token", func(t *testing.T) {
		q.Operation, q.Source = service.ReadObservation, nil
		q.Minimum = &service.Token{Scope: q.Scope, Repository: q.Repository, Kind: mirror.Issues, Value: "required-observation"}
		r.Operation, r.Source = q.Operation, nil
		r.Metadata.Observation = &service.Token{Scope: q.Scope, Repository: q.Repository, Kind: mirror.Issues, Value: "other-observation"}
		// This is otherwise structurally valid v1. A different confirmed token
		// must not satisfy a minimum-progress read.
		wrong, _ := json.Marshal(r)
		if _, err := service.DecodeResponse(wrong, q); !errors.Is(err, service.ErrScope) {
			t.Fatal("wrong observation token accepted", err)
		}
		r.Metadata.Observation.Value = q.Minimum.Value
		if _, err := service.EncodeResponse(r, q); err != nil {
			t.Fatal("matching token refused", err)
		}
		r.Metadata.Observation.Scope.AccessEpoch = "other-epoch"
		wrong, _ = json.Marshal(r)
		if _, err := service.DecodeResponse(wrong, q); err == nil {
			t.Fatal("cross-epoch observation accepted")
		}
	})
	t.Run("legacy", func(t *testing.T) {
		res, err := forgeread.ParseIssuesEnvelope(fixture(t, "legacy-issues.json"), []string{"example/project"})
		if err != nil || len(res.Unavailable) != 0 || res.Issues["example/project"] == nil || len(res.Issues["example/project"]) != 0 {
			t.Fatal("known-empty legacy result changed", err)
		}
		frozen, err := forgeread.NewFrozenReader(res).OpenIssues(context.Background(), []string{"example/project", "example/missing"})
		if err != nil || len(frozen.Issues) != 1 || len(frozen.Unavailable) != 1 || frozen.Unavailable[0].Class != forgeread.ClassMissing {
			t.Fatal("legacy frozen missing became empty", err)
		}
		if _, err := forgeread.ParseIssuesEnvelope([]byte(`{"schema":999,"kind":"issues","repos":[],"partial":[]}`), nil); !errors.Is(err, forgeread.ErrUnsupportedSchema) {
			t.Fatal("unknown legacy schema accepted")
		}
	})
	t.Run("held", func(t *testing.T) {
		q := literalRequest(t)
		for _, reason := range []mirror.Reason{mirror.Permission, mirror.Transport, mirror.RateLimited, mirror.Truncated, mirror.Unknown} {
			h := service.Held(q, reason)
			if _, err := service.EncodeResponse(h, q); err != nil {
				t.Fatal("explicit hold refused", err)
			}
			h.Source = literalResponse(t, q).Source
			if _, err := service.EncodeResponse(h, q); err == nil {
				t.Fatal("held result leaked private data")
			}
		}
	})
}
