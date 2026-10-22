package registrytest

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"

	"github.com/auth0/auth0-cli/internal/plugins"
	"github.com/auth0/auth0-cli/internal/plugins/registry"
)

// ArchiveEntry is one entry of a test archive. The zero Type is a regular file.
//
// The builders write whatever they are given, including names and entry types
// that the CLI refuses, so that a test can serve a hostile archive.
type ArchiveEntry struct {
	Name string
	Body []byte

	// Type is the tar type flag, such as tar.TypeSymlink. A zip archive has no
	// hard links, so TypeLink is not available there. The other types become the
	// matching file mode of the zip entry.
	Type byte

	// Link is the target of a link entry.
	Link string
}

// TarGz returns a tar.gz archive with the entries in the given order.
func TarGz(entries ...ArchiveEntry) []byte {
	var buffer bytes.Buffer

	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)

	for _, entry := range entries {
		typeflag := entry.Type
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}

		header := &tar.Header{Name: entry.Name, Typeflag: typeflag, Linkname: entry.Link, Mode: 0o755}
		if typeflag == tar.TypeXGlobalHeader {
			header = &tar.Header{Name: entry.Name, Typeflag: typeflag, PAXRecords: map[string]string{"comment": "test"}}
		}

		if typeflag == tar.TypeReg {
			header.Size = int64(len(entry.Body))
		}

		must(archive.WriteHeader(header))

		if typeflag == tar.TypeReg {
			_, err := archive.Write(entry.Body)
			must(err)
		}
	}

	must(archive.Close())
	must(compressed.Close())

	return buffer.Bytes()
}

// Zip returns a zip archive with the entries in the given order.
func Zip(entries ...ArchiveEntry) []byte {
	var buffer bytes.Buffer

	archive := zip.NewWriter(&buffer)

	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate}

		body := entry.Body

		switch entry.Type {
		case tar.TypeSymlink:
			header.SetMode(fs.ModeSymlink | 0o777)

			body = []byte(entry.Link)
		case tar.TypeDir:
			header.SetMode(fs.ModeDir | 0o755)

			body = nil
		case tar.TypeChar:
			header.SetMode(fs.ModeDevice | fs.ModeCharDevice | 0o644)
		case tar.TypeFifo:
			header.SetMode(fs.ModeNamedPipe | 0o644)
		default:
			header.SetMode(0o755)
		}

		writer, err := archive.CreateHeader(header)
		must(err)

		_, err = writer.Write(body)
		must(err)
	}

	must(archive.Close())

	return buffer.Bytes()
}

// Manifest returns a valid plugin.json for a version of the plugin that matches
// the record: its version, platforms, engines, commands and permissions. The
// permissions go on the first command.
func Manifest(id string, record registry.VersionRecord) []byte {
	type command struct {
		Name        string               `json:"name"`
		Description string               `json:"description"`
		Interactive bool                 `json:"interactive"`
		Permissions *plugins.Permissions `json:"permissions,omitempty"`
	}

	commands := make([]command, 0, len(record.Commands))

	for n, consent := range record.Commands {
		item := command{Name: consent.Name, Description: "Run " + consent.Name, Interactive: consent.Interactive}
		if n == 0 {
			permissions := record.Permissions
			item.Permissions = &permissions
		}

		commands = append(commands, item)
	}

	manifest := map[string]any{
		"id":          id,
		"version":     record.Version,
		"apiVersion":  1,
		"description": "Test plugin " + id,
		"owner":       plugins.Owner{Team: "Test team", Bugs: "https://example.com/bugs"},
		"platforms":   record.Platforms,
		"commands":    commands,
	}

	if record.Engines != nil {
		manifest["engines"] = record.Engines
	}

	data, err := json.Marshal(manifest)
	must(err)

	return data
}

// SHA256Hex returns the lowercase hex SHA-256 of data, which is how the index
// and the lockfile write a hash.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

// NewArtifact returns the artifact record for an archive that is served at url,
// with the hash and size of the archive. The bin is the name of the executable
// inside the archive.
func NewArtifact(platform plugins.Platform, url string, format registry.ArtifactFormat, bin string, archive []byte) registry.Artifact {
	return registry.Artifact{
		OS:     platform.OS(),
		Arch:   platform.Arch(),
		Libc:   platform.Libc(),
		URL:    url,
		SHA256: SHA256Hex(archive),
		Size:   int64(len(archive)),
		Format: format,
		Bin:    bin,
	}
}

// Package returns an archive of the given format that holds the binary and the
// manifest of the record, under the names bin and plugin.json.
func Package(id string, record registry.VersionRecord, format registry.ArtifactFormat, bin string, binary []byte) []byte {
	entries := []ArchiveEntry{{Name: bin, Body: binary}, {Name: "plugin.json", Body: Manifest(id, record)}}

	if format == registry.ArtifactFormatZip {
		return Zip(entries...)
	}

	return TarGz(entries...)
}

// must panics on an error that cannot happen when writing to memory.
func must(err error) {
	if err != nil {
		panic(err)
	}
}
