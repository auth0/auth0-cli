package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	if err := os.WriteFile(src, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", string(data))
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	err := CopyFile(filepath.Join(tmpDir, "nonexistent.txt"), filepath.Join(tmpDir, "dst.txt"))
	if err == nil {
		t.Fatal("expected error for missing source file")
	}
}

func TestFetchKeys_SortedOrder(t *testing.T) {
	m := map[string]int{"banana": 1, "apple": 2, "cherry": 3}
	keys := FetchKeys(m)
	expected := []string{"apple", "banana", "cherry"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("expected keys[%d] = %q, got %q", i, expected[i], k)
		}
	}
}

func TestFetchKeys_EmptyMap(t *testing.T) {
	keys := FetchKeys(map[string]int{})
	if len(keys) != 0 {
		t.Errorf("expected empty slice, got %v", keys)
	}
}

func TestCopyFile_DestDirNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")

	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	err := CopyFile(src, filepath.Join(tmpDir, "nonexistent", "dst.txt"))
	if err == nil {
		t.Fatal("expected error when destination directory does not exist")
	}
}
