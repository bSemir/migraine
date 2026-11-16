package fsys_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bSemir/migraine/internal/adapters/fsys"
	"github.com/bSemir/migraine/internal/core/domain"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestListSkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "000001_init.up.sql"), "")
	write(t, filepath.Join(dir, "notes.txt"), "")
	if err := os.Mkdir(filepath.Join(dir, "archive"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := fsys.New().List(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"000001_init.up.sql", "notes.txt"}; !slices.Equal(got, want) {
		t.Errorf("List = %v, want %v", got, want)
	}
}

func TestListMissingDir(t *testing.T) {
	_, err := fsys.New().List(t.Context(), filepath.Join(t.TempDir(), "nope"))
	if !errors.Is(err, domain.ErrDirNotFound) {
		t.Fatalf("err = %v, want ErrDirNotFound", err)
	}
}

func TestListOnFileIsAnError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	write(t, file, "")
	_, err := fsys.New().List(t.Context(), file)
	if err == nil || errors.Is(err, domain.ErrDirNotFound) {
		t.Fatalf("err = %v, want a read error", err)
	}
}

func TestRenameAndExists(t *testing.T) {
	dir := t.TempDir()
	oldPath, newPath := filepath.Join(dir, "a.sql"), filepath.Join(dir, "b.sql")
	write(t, oldPath, "select 1;")
	f := fsys.New()

	if err := f.Rename(t.Context(), oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{oldPath: false, newPath: true} {
		got, err := f.Exists(t.Context(), path)
		if err != nil || got != want {
			t.Errorf("Exists(%s) = %v, %v; want %v", filepath.Base(path), got, err, want)
		}
	}
	if content, _ := os.ReadFile(newPath); string(content) != "select 1;" {
		t.Errorf("content after rename = %q", content)
	}
}

func TestRenameMissingSourceFails(t *testing.T) {
	dir := t.TempDir()
	err := fsys.New().Rename(t.Context(), filepath.Join(dir, "missing"), filepath.Join(dir, "x"))
	if err == nil {
		t.Fatal("want an error renaming a missing file")
	}
}

func TestCancelledContextStopsEveryOperation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	f := fsys.New()

	if _, err := f.List(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Errorf("List err = %v", err)
	}
	if err := f.Rename(ctx, "a", "b"); !errors.Is(err, context.Canceled) {
		t.Errorf("Rename err = %v", err)
	}
	if _, err := f.Exists(ctx, "a"); !errors.Is(err, context.Canceled) {
		t.Errorf("Exists err = %v", err)
	}
}
