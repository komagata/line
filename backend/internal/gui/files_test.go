package gui

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func fileURL(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }
func TestSnapshotRejectsSymlinkAndSpecialFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.WriteFile(target, []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotFile(fileURL(filepath.Join(dir, "link"))); err == nil {
		t.Fatal("symlink accepted")
	}
	if err := unix.Mkfifo(filepath.Join(dir, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotFile(fileURL(filepath.Join(dir, "fifo"))); err == nil {
		t.Fatal("fifo accepted")
	}
}
func TestDownloadPublicationNeverOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "keep")
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	fd, name, err := openParent(fileURL(target))
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if err := publishFile(context.Background(), fd, name, []byte("new")); err == nil {
		t.Fatal("overwrote existing target")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "old" {
		t.Fatalf("target changed: %s %v", data, err)
	}
}
func TestHeldDirectoryCannotBeRetargetedDuringDownload(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	os.Mkdir(a, 0700)
	os.Mkdir(b, 0700)
	fd, name, err := openParent(fileURL(filepath.Join(a, "new")))
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if err := os.Rename(a, filepath.Join(root, "old-a")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(b, a); err != nil {
		t.Fatal(err)
	}
	if err := publishFile(context.Background(), fd, name, []byte("safe")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b, "new")); !os.IsNotExist(err) {
		t.Fatal("published into swapped directory")
	}
}
func TestDownloadGuardPreventsLatePublication(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cancelled")
	fd, name, err := openParent(fileURL(target))
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	called := false
	err = publishFile(context.Background(), fd, name, []byte("data"), func(link func() error) error { called = true; return context.Canceled })
	if !called || err == nil {
		t.Fatal("publication guard was bypassed")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("cancelled target exists")
	}
}
func TestFileURLRejectsBidiControl(t *testing.T) {
	if _, err := filePath(fileURL("/tmp/report\u202eexe")); err == nil {
		t.Fatal("bidi control accepted")
	}
}
