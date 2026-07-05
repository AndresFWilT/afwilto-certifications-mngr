package fsstore

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestPutGetDeleteRoundTrip(t *testing.T) {
	store := NewDiskFileStorage(t.TempDir())
	ctx := context.Background()

	if err := store.Put(ctx, "certs/c1/diploma", strings.NewReader("pdf-bytes")); err != nil {
		t.Fatal(err)
	}
	r, err := store.Get(ctx, "certs/c1/diploma")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	r.Close()
	if string(got) != "pdf-bytes" {
		t.Errorf("content = %q", got)
	}

	if err := store.Delete(ctx, "certs/c1/diploma"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "certs/c1/diploma"); err == nil {
		t.Error("get after delete succeeded")
	}
	// Deleting a missing key is a no-op, not an error.
	if err := store.Delete(ctx, "certs/c1/diploma"); err != nil {
		t.Errorf("second delete = %v", err)
	}
}

func TestPutOverwritesAtomically(t *testing.T) {
	store := NewDiskFileStorage(t.TempDir())
	ctx := context.Background()
	_ = store.Put(ctx, "k", strings.NewReader("old"))
	if err := store.Put(ctx, "k", strings.NewReader("new")); err != nil {
		t.Fatal(err)
	}
	r, _ := store.Get(ctx, "k")
	got, _ := io.ReadAll(r)
	r.Close()
	if string(got) != "new" {
		t.Errorf("content = %q, want replacement", got)
	}
}

func TestRejectsPathEscape(t *testing.T) {
	store := NewDiskFileStorage(t.TempDir())
	if err := store.Put(context.Background(), "../escape", strings.NewReader("x")); err == nil {
		t.Error("path escape accepted")
	}
}
