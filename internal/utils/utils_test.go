package utils

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createZip(t *testing.T, zipPath string, files map[string]string) {
	out, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	zipWriter := zip.NewWriter(out)
	defer zipWriter.Close()

	for name, content := range files {
		w, err := zipWriter.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUnzip_Success(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	destDir := filepath.Join(tmpDir, "out")

	files := map[string]string{
		"file1.txt":         "hello",
		"dir/file2.txt":     "world",
		"dir/sub/file3.txt": "nested",
	}
	createZip(t, zipPath, files)

	err := Unzip(zipPath, destDir)
	if err != nil {
		t.Fatalf("Unzip failed: %v", err)
	}

	for name, content := range files {
		fullPath := filepath.Join(destDir, name)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("failed to read %s: %v", name, err)
			continue
		}
		if string(data) != content {
			t.Errorf("expected content %q in %s but got %q", content, name, string(data))
		}
	}
}

func TestUnzip_IllegalPath(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "bad.zip")
	destDir := filepath.Join(tmpDir, "out")

	out, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	zipWriter := zip.NewWriter(out)
	_, err = zipWriter.Create("../evil.txt") // zip-slip attack
	if err != nil {
		t.Fatal(err)
	}
	zipWriter.Close()

	err = Unzip(zipPath, destDir)
	if err == nil || !strings.Contains(err.Error(), "illegal file path") {
		t.Fatalf("expected zip-slip error, got: %v", err)
	}
}

func TestUnzip_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	invalidZip := filepath.Join(tmpDir, "not_a_zip.txt")

	if err := os.WriteFile(invalidZip, []byte("invalid data"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Unzip(invalidZip, tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid zip file")
	}
}
