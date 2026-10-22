package store

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

const (
	// MaxArchiveEntries is how many entries an archive has at most: the
	// executable and plugin.json.
	maxArchiveEntries = 2

	// TarOverheadBytes is how much more than the files themselves the unpacked
	// stream of a tar archive may hold, for its headers and padding.
	tarOverheadBytes = 4 << 20
)

// extracted is what was unpacked from an archive.
type extracted struct {
	binSHA256      string
	binSize        int64
	manifestSHA256 string
	manifest       []byte
}

// archiveEntry is one entry of an archive, as the extraction sees it.
type archiveEntry struct {
	name string

	// Kind describes what the entry is when it is not a regular file, and is
	// empty for a regular file.
	kind string

	// Open returns the content of the entry. It may only be used before the next
	// entry is read.
	open func() (io.ReadCloser, error)
}

// nextEntry returns the next entry of an archive, or io.EOF after the last.
type nextEntry func() (archiveEntry, error)

// extract unpacks the archive in the open file, which is read from its start,
// into the new directory dest. The archive must hold exactly two regular files
// at its root, the executable named bin and plugin.json, and nothing else: no
// directories, links, devices or other special entries, no second file with the
// same name, and no name that is not a plain file name. What is unpacked is at
// most maxBytes in total.
//
// Nothing in an archive decides where a file is written: the two names are the
// only ones accepted, and each is created in dest under that name. The
// executable gets the mode 0755 and plugin.json the mode 0600.
func extract(file *os.File, format registry.ArtifactFormat, bin, dest string, maxBytes int64) (extracted, error) {
	if reason := badFileName(bin); reason != "" || strings.EqualFold(bin, manifestFileName) {
		return extracted{}, fmt.Errorf("%w (the registry lists %q as the executable, which is not a plain file name that differs from %s)", ErrInvalidArchive, bin, manifestFileName)
	}

	if err := os.Mkdir(dest, dirPerm); err != nil {
		return extracted{}, fmt.Errorf("failed to create %s: %w", dest, err)
	}

	var (
		next    nextEntry
		counter *io.LimitedReader
		err     error
	)

	switch format {
	case registry.ArtifactFormatTarGz:
		next, counter, err = tarEntries(file, maxBytes)
	case registry.ArtifactFormatZip:
		next, err = zipEntries(file)
	default:
		err = fmt.Errorf("%w (the format %q cannot be unpacked)", ErrInvalidArchive, format)
	}

	if err != nil {
		return extracted{}, err
	}

	result, err := unpackEntries(next, bin, dest, maxBytes)
	if err != nil && counter != nil && counter.N <= 0 {
		return extracted{}, fmt.Errorf("%w (the archive unpacks to more than the allowed %d bytes)", ErrInvalidArchive, maxBytes)
	}

	return result, err
}

// unpackEntries writes the two files of the archive into dest.
func unpackEntries(next nextEntry, bin, dest string, maxBytes int64) (extracted, error) {
	var (
		result      extracted
		seen        = map[string]bool{}
		remaining   = maxBytes
		entriesRead int
	)

	for {
		entry, err := next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return extracted{}, fmt.Errorf("%w (%w)", ErrInvalidArchive, err)
		}

		entriesRead++

		reason := badFileName(entry.name)

		switch {
		case entriesRead > maxArchiveEntries:
			return extracted{}, fmt.Errorf("%w (it has more than the %d entries a plugin archive has)", ErrInvalidArchive, maxArchiveEntries)
		case reason != "":
			return extracted{}, fmt.Errorf("%w (the entry %q is not a plain file name: %s)", ErrInvalidArchive, entry.name, reason)
		case entry.kind != "":
			return extracted{}, fmt.Errorf("%w (the entry %q is a %s, and only regular files are allowed)", ErrInvalidArchive, entry.name, entry.kind)
		case entry.name != bin && entry.name != manifestFileName:
			return extracted{}, fmt.Errorf("%w (the entry %q is neither %s nor %s)", ErrInvalidArchive, entry.name, bin, manifestFileName)
		case seen[entry.name]:
			return extracted{}, fmt.Errorf("%w (the entry %q is there twice)", ErrInvalidArchive, entry.name)
		}

		seen[entry.name] = true

		limit := remaining
		if entry.name == manifestFileName {
			limit = min(limit, maxManifestBytes)
		}

		sum, size, content, err := writeEntry(entry, filepath.Join(dest, entry.name), limit)
		if err != nil {
			return extracted{}, err
		}

		remaining -= size

		if entry.name == bin {
			result.binSHA256, result.binSize = sum, size
		} else {
			result.manifestSHA256, result.manifest = sum, content
		}
	}

	if !seen[bin] || !seen[manifestFileName] {
		return extracted{}, fmt.Errorf("%w (it must hold both %s and %s)", ErrInvalidArchive, bin, manifestFileName)
	}

	return result, nil
}

// writeEntry creates the file at path, which must not exist, with the content of
// the entry, which must not be empty or longer than limit bytes. It returns the
// SHA-256 and size of the content, and the content itself for plugin.json.
func writeEntry(entry archiveEntry, path string, limit int64) (sum string, size int64, manifest []byte, err error) {
	source, err := entry.open()
	if err != nil {
		return "", 0, nil, fmt.Errorf("%w (%q cannot be read: %w)", ErrInvalidArchive, entry.name, err)
	}

	defer source.Close()

	mode := binaryPerm
	if entry.name == manifestFileName {
		mode = filePerm
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
	if err != nil {
		return "", 0, nil, fmt.Errorf("failed to create %s: %w", path, err)
	}

	hasher := sha256.New()
	writers := []io.Writer{file, hasher}

	var content bytes.Buffer

	if entry.name == manifestFileName {
		writers = append(writers, &content)
	}

	size, err = io.Copy(io.MultiWriter(writers...), io.LimitReader(source, limit+1))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}

	switch {
	case err != nil:
		return "", 0, nil, fmt.Errorf("%w (%q cannot be unpacked: %w)", ErrInvalidArchive, entry.name, err)
	case size > limit:
		return "", 0, nil, fmt.Errorf("%w (%q is larger than the allowed %d bytes)", ErrInvalidArchive, entry.name, limit)
	case size == 0:
		return "", 0, nil, fmt.Errorf("%w (%q is empty)", ErrInvalidArchive, entry.name)
	}

	if err := os.Chmod(path, mode); err != nil {
		return "", 0, nil, fmt.Errorf("failed to set the mode of %s: %w", path, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), size, content.Bytes(), nil
}

// windowsDeviceNames are the names that Windows reserves for devices, with or
// without an extension, in any case.
var windowsDeviceNames = []string{
	"con", "prn", "aux", "nul",
	"com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
	"lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9",
}

// badFileName says why name is not a plain file name, or returns an empty string
// when it is one. A plain file name has no directory part in either style, no
// drive letter, no control characters, no dot or space at its end, and is not
// "." or "..", or a name that Windows reserves for a device. The rules do not
// depend on the system, so that a plugin that installs here can be installed
// everywhere.
func badFileName(name string) string {
	stem, _, _ := strings.Cut(name, ".")

	switch {
	case name == "" || name == "." || name == "..":
		return "it names no file"
	case strings.HasSuffix(name, ".") || strings.HasSuffix(name, " "):
		return "it ends with a dot or a space"
	case slices.Contains(windowsDeviceNames, strings.ToLower(strings.TrimRight(stem, " "))):
		return "it is a device name on Windows"
	case strings.ContainsAny(name, `/\`):
		return "it has a directory part"
	case strings.Contains(name, ":"):
		return "it has a drive letter or a stream name"
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return "it has a control character"
	}

	return ""
}

// tarEntries reads the entries of a gzip compressed tar archive. The unpacked
// stream is capped at maxBytes plus room for the headers, and the returned
// reader tells whether the cap was reached.
func tarEntries(file io.Reader, maxBytes int64) (nextEntry, *io.LimitedReader, error) {
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return nil, nil, fmt.Errorf("%w (it is not gzip compressed: %w)", ErrInvalidArchive, err)
	}

	capped := &io.LimitedReader{R: compressed, N: maxBytes + tarOverheadBytes}
	reader := tar.NewReader(capped)

	return func() (archiveEntry, error) {
		header, err := reader.Next()
		if err != nil {
			return archiveEntry{}, err
		}

		return archiveEntry{
			name: header.Name,
			kind: tarKind(header.Typeflag),
			open: func() (io.ReadCloser, error) { return io.NopCloser(reader), nil },
		}, nil
	}, capped, nil
}

// tarKind describes a tar entry type, and returns an empty string for a regular
// file.
func tarKind(typeflag byte) string {
	switch typeflag {
	case tar.TypeReg:
		return ""
	case tar.TypeLink:
		return "hard link"
	case tar.TypeSymlink:
		return "symbolic link"
	case tar.TypeDir:
		return "directory"
	case tar.TypeChar:
		return "character device"
	case tar.TypeBlock:
		return "block device"
	case tar.TypeFifo:
		return "named pipe"
	default:
		return fmt.Sprintf("special entry of type %q", typeflag)
	}
}

// zipEntries reads the entries of a zip archive in the file.
func zipEntries(file *os.File) (nextEntry, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to read the download: %w", err)
	}

	reader, err := zip.NewReader(file, info.Size())
	if err != nil {
		return nil, fmt.Errorf("%w (it is not a zip archive: %w)", ErrInvalidArchive, err)
	}

	files := reader.File

	return func() (archiveEntry, error) {
		if len(files) == 0 {
			return archiveEntry{}, io.EOF
		}

		current := files[0]
		files = files[1:]

		return archiveEntry{
			name: current.Name,
			kind: zipKind(current),
			open: current.Open,
		}, nil
	}, nil
}

// zipKind describes a zip entry that is not a regular file, and returns an empty
// string for a regular file.
func zipKind(file *zip.File) string {
	mode := file.Mode()

	switch {
	case mode.IsRegular():
		return ""
	case mode.IsDir():
		return "directory"
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	case mode&os.ModeNamedPipe != 0:
		return "named pipe"
	case mode&os.ModeDevice != 0:
		return "device"
	default:
		return "special file"
	}
}
