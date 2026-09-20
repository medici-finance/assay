package workpad

import (
	"reflect"
	"testing"
)

func TestUpsertAppendsWhenNoOwnComment(t *testing.T) {
	got := Upsert([]string{"a", "b"}, -1, "c")
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUpsertReplacesOwnCommentInPlace(t *testing.T) {
	got := Upsert([]string{"a", "old-workpad", "c"}, 1, "new-workpad")
	want := []string{"a", "new-workpad", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUpsertDoesNotMutateInput(t *testing.T) {
	in := []string{"a", "old-workpad"}
	_ = Upsert(in, 1, "new-workpad")
	if in[1] != "old-workpad" {
		t.Fatalf("input slice was mutated: %v", in)
	}
}
