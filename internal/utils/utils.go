package utils

import (
	"fmt"
	"io"
	"os"
	"strings"

	"archive/zip"
	"path/filepath"
)

func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}

	defer func(r *zip.ReadCloser) {
		_ = r.Close()
	}(r)

	for _, f := range r.File {
		filPath := filepath.Join(dest, f.Name)

		relPath, err := filepath.Rel(dest, filPath)
		if err != nil || strings.Contains(relPath, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", filPath)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(filPath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(filPath), os.ModePerm); err != nil {
			return err
		}

		inFile, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(filPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			_ = inFile.Close()
			return err
		}

		_, err = io.Copy(outFile, inFile)
		_ = inFile.Close()
		_ = outFile.Close()

		if err != nil {
			return err
		}
	}

	return nil
}
