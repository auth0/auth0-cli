package store_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/auth0/auth0-cli/internal/plugins/registry"
	"github.com/auth0/auth0-cli/internal/plugins/registry/registrytest"
	"github.com/auth0/auth0-cli/internal/plugins/store"
)

// withEntries makes a release whose archive holds exactly the entries that
// build returns.
func withEntries(format registry.ArtifactFormat, build func(r *release) []registrytest.ArchiveEntry) func(*release) {
	return func(r *release) {
		r.format = format

		r.archive = func(r *release) []byte {
			if format == registry.ArtifactFormatZip {
				return registrytest.Zip(build(r)...)
			}

			return registrytest.TarGz(build(r)...)
		}
	}
}

func TestExtract_RefusesUnsafeArchives(t *testing.T) {
	type archiveCase struct {
		name    string
		entries func(r *release) []registrytest.ArchiveEntry
		want    string

		// Formats limits the case to the formats that can hold the entry.
		formats []registry.ArtifactFormat
	}

	both := []registry.ArtifactFormat{registry.ArtifactFormatTarGz, registry.ArtifactFormatZip}
	tarOnly := []registry.ArtifactFormat{registry.ArtifactFormatTarGz}

	manifestOf := func(r *release) registrytest.ArchiveEntry {
		return registrytest.ArchiveEntry{Name: "plugin.json", Body: registrytest.Manifest(testID, r.record)}
	}

	binOf := func(r *release) registrytest.ArchiveEntry {
		return registrytest.ArchiveEntry{Name: r.bin, Body: r.binary}
	}

	tests := []archiveCase{
		{"a path that leaves the directory", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{{Name: "../" + r.bin, Body: r.binary}, manifestOf(r)}
		}, "not a plain file name", both},
		{"a symbolic link as the executable", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{{Name: r.bin, Type: tar.TypeSymlink, Link: "/bin/sh"}, manifestOf(r)}
		}, "symbolic link", both},
		{"a hard link", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{binOf(r), {Name: "plugin.json", Type: tar.TypeLink, Link: r.bin}}
		}, "hard link", tarOnly},
		{"a character device", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{{Name: r.bin, Type: tar.TypeChar}, manifestOf(r)}
		}, "device", both},
		{"a block device", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{{Name: r.bin, Type: tar.TypeBlock}, manifestOf(r)}
		}, "block device", tarOnly},
		{"a named pipe", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{{Name: r.bin, Type: tar.TypeFifo}, manifestOf(r)}
		}, "named pipe", both},
		{"a directory", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{binOf(r), {Name: "plugin.json", Type: tar.TypeDir}}
		}, "directory", both},
		{"an extended header entry", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{{Name: "pax_global_header", Type: tar.TypeXGlobalHeader}, binOf(r), manifestOf(r)}
		}, "special entry", tarOnly},
		{"the executable twice", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{binOf(r), binOf(r)}
		}, "there twice", both},
		{"the manifest twice", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{manifestOf(r), manifestOf(r)}
		}, "there twice", both},
		{"an extra file", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{binOf(r), {Name: "README.md", Body: []byte("hi")}}
		}, "neither", both},
		{"an extra file after the two", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{binOf(r), manifestOf(r), {Name: "README.md", Body: []byte("hi")}}
		}, "more than the 2 entries", both},
		{"a name that differs only in case", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{binOf(r), {Name: "Plugin.json", Body: registrytest.Manifest(testID, r.record)}}
		}, "neither", both},
		{"no executable", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{manifestOf(r)}
		}, "must hold both", both},
		{"no manifest", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{binOf(r)}
		}, "must hold both", both},
		{"an executable that is named differently", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{{Name: "other", Body: r.binary}, manifestOf(r)}
		}, "neither", both},
		{"an empty archive", func(*release) []registrytest.ArchiveEntry { return nil }, "must hold both", both},
		{"an empty executable", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{{Name: r.bin}, manifestOf(r)}
		}, "is empty", both},
		{"a manifest that is too large", func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{binOf(r), {Name: "plugin.json", Body: repeated(' ', 2<<20)}}
		}, "is larger than the allowed", both},
	}

	for _, test := range tests {
		for _, format := range test.formats {
			t.Run(test.name+" in "+string(format), func(t *testing.T) {
				f := newFixture(t)
				index := f.index(f.release("1.0.0", withEntries(format, test.entries)))

				_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

				require.ErrorIs(t, err, store.ErrInvalidArchive)
				assert.Contains(t, err.Error(), test.want)
				f.assertNothingInstalled(t)
			})
		}
	}
}

func TestExtract_RefusesAnArchiveThatExpandsPastTheLimit(t *testing.T) {
	for _, format := range []registry.ArtifactFormat{registry.ArtifactFormatTarGz, registry.ArtifactFormatZip} {
		t.Run(string(format), func(t *testing.T) {
			f := newFixture(t)
			f.policy = registry.Policy{MaxDownloadBytes: 1 << 20}

			// Two megabytes of zeros compress to a few kilobytes, far below the limit
			// of the download, but the limit applies to what is unpacked as well.
			bomb := func(r *release) { r.binary = repeated(0, 2<<20) }

			record := f.release("1.0.0", bomb, withEntries(format, goodEntries))
			require.Less(t, record.Artifacts[0].Size, int64(100<<10), "the archive itself is small")

			_, err := f.store.Install(t.Context(), f.index(record), testID, "", store.Options{})

			require.ErrorIs(t, err, store.ErrInvalidArchive)
			assert.Contains(t, err.Error(), "allowed")
			f.assertNothingInstalled(t)
		})
	}

	t.Run("also when the entries add up to more than the limit", func(t *testing.T) {
		f := newFixture(t)
		f.policy = registry.Policy{MaxDownloadBytes: 1 << 20}

		// Neither file is over the limit alone, but together they are.
		index := f.index(f.release("1.0.0", func(r *release) { r.binary = repeated(1, 900<<10) }, withEntries(registry.ArtifactFormatTarGz, func(r *release) []registrytest.ArchiveEntry {
			return []registrytest.ArchiveEntry{{Name: r.bin, Body: r.binary}, {Name: "plugin.json", Body: append(registrytest.Manifest(testID, r.record), repeated(' ', 300<<10)...)}}
		})))

		_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

		assert.ErrorIs(t, err, store.ErrInvalidArchive)
	})
}

func TestExtract_RefusesFilesThatAreNotArchives(t *testing.T) {
	notAnArchive := func(r *release) []byte { return []byte("this is not an archive") }

	truncated := func(r *release) []byte {
		archive := registrytest.Package(testID, r.record, r.format, r.bin, r.binary)

		return archive[:len(archive)-40]
	}

	plainTar := func(r *release) []byte {
		var buffer bytes.Buffer

		archive := tar.NewWriter(&buffer)
		require.NoError(t, archive.WriteHeader(&tar.Header{Name: r.bin, Size: 1, Mode: 0o755}))
		_, err := archive.Write([]byte("x"))
		require.NoError(t, err)
		require.NoError(t, archive.Close())

		return buffer.Bytes()
	}

	garbageGzip := func(r *release) []byte {
		var buffer bytes.Buffer

		writer := gzip.NewWriter(&buffer)
		_, err := writer.Write(repeated('x', 2048))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		return buffer.Bytes()
	}

	tests := []struct {
		name    string
		format  registry.ArtifactFormat
		archive func(r *release) []byte
	}{
		{"bytes that are neither gzip nor tar", registry.ArtifactFormatTarGz, notAnArchive},
		{"a tar that is not compressed", registry.ArtifactFormatTarGz, plainTar},
		{"gzip data that is not a tar", registry.ArtifactFormatTarGz, garbageGzip},
		{"a truncated tar.gz", registry.ArtifactFormatTarGz, truncated},
		{"bytes that are not a zip", registry.ArtifactFormatZip, notAnArchive},
		{"a truncated zip", registry.ArtifactFormatZip, truncated},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture(t)
			index := f.index(f.release("1.0.0", func(r *release) {
				r.format = test.format
				r.archive = test.archive
			}))

			_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

			require.ErrorIs(t, err, store.ErrInvalidArchive)
			f.assertNothingInstalled(t)
		})
	}
}

func TestExtract_RefusesAnExecutableNamedLikeTheManifest(t *testing.T) {
	for _, bin := range []string{"plugin.json", "Plugin.JSON"} {
		t.Run(bin, func(t *testing.T) {
			f := newFixture(t)
			index := f.index(f.release("1.0.0", func(r *release) { r.bin = bin }))

			_, err := f.store.Install(t.Context(), index, testID, "", store.Options{})

			require.ErrorIs(t, err, store.ErrInvalidArchive)
			assert.Empty(t, f.installedVersions())
		})
	}
}
