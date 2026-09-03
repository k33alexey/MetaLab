package publication

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/k33alexey/MetaLab/internal/project"
)

const maxPackageManifestBytes = 16 << 20

// VerifyFile validates the package format, archive paths and every source digest.
func VerifyFile(ctx context.Context, packagePath string) (Manifest, error) {
	archive, err := zip.OpenReader(packagePath)
	if err != nil {
		return Manifest{}, fmt.Errorf("open publication package: %w", err)
	}
	defer archive.Close()
	if len(archive.File) == 0 || archive.File[0].Name != "package.json" {
		return Manifest{}, fmt.Errorf("publication package manifest is missing")
	}
	manifest, err := readPackageManifest(archive.File[0])
	if err != nil {
		return Manifest{}, err
	}
	if len(archive.File) != len(manifest.Files)+1 {
		return Manifest{}, fmt.Errorf("publication package entries do not match its manifest")
	}
	contentHash := sha256.New()
	var total int64
	previous := ""
	manifestSourceFound := false
	for index, expected := range manifest.Files {
		if err := ctx.Err(); err != nil {
			return Manifest{}, err
		}
		if err := validatePackagePath(expected.Path); err != nil {
			return Manifest{}, err
		}
		if expected.Path == project.ManifestFile {
			manifestSourceFound = true
		} else if err := validateSourcePath(expected.Path, false); err != nil {
			return Manifest{}, err
		}
		if previous != "" && expected.Path <= previous {
			return Manifest{}, fmt.Errorf("publication package files must be unique and sorted")
		}
		previous = expected.Path
		entry := archive.File[index+1]
		if entry.Name != expected.Path || !entry.Mode().IsRegular() || int64(entry.UncompressedSize64) != expected.Size || expected.Size < 0 || expected.Size > maxSourceFileBytes {
			return Manifest{}, fmt.Errorf("publication entry %q does not match its manifest", expected.Path)
		}
		total += expected.Size
		if total > maxPackageInputBytes {
			return Manifest{}, fmt.Errorf("publication package sources exceed %d bytes", maxPackageInputBytes)
		}
		digest, err := digestZipEntry(ctx, entry, expected.Size)
		if err != nil {
			return Manifest{}, err
		}
		if digest != expected.SHA256 {
			return Manifest{}, fmt.Errorf("publication entry %q checksum mismatch", expected.Path)
		}
		_, _ = fmt.Fprintf(contentHash, "%s\x00%d\x00%s\n", expected.Path, expected.Size, expected.SHA256)
	}
	if !manifestSourceFound {
		return Manifest{}, fmt.Errorf("publication package does not contain %s", project.ManifestFile)
	}
	if hex.EncodeToString(contentHash.Sum(nil)) != manifest.ContentSHA256 {
		return Manifest{}, fmt.Errorf("publication package content checksum mismatch")
	}
	return manifest, nil
}

func readPackageManifest(entry *zip.File) (Manifest, error) {
	if entry.UncompressedSize64 > maxPackageManifestBytes {
		return Manifest{}, fmt.Errorf("publication package manifest is too large")
	}
	file, err := entry.Open()
	if err != nil {
		return Manifest{}, fmt.Errorf("open publication package manifest: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(bufio.NewReader(io.LimitReader(file, maxPackageManifestBytes+1)))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode publication package manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("publication package manifest must contain one JSON document")
	}
	if manifest.Format != CurrentPackageFormat || manifest.ProjectFormat != project.CurrentFormat || manifest.ProjectID.IsZero() || strings.TrimSpace(manifest.ProjectName) == "" {
		return Manifest{}, fmt.Errorf("unsupported or invalid publication package manifest")
	}
	if err := validateGitCommit(manifest.GitCommit); err != nil {
		return Manifest{}, err
	}
	if len(manifest.ContentSHA256) != sha256.Size*2 {
		return Manifest{}, fmt.Errorf("invalid publication package content checksum")
	}
	return manifest, nil
}

func digestZipEntry(ctx context.Context, entry *zip.File, expectedSize int64) (string, error) {
	file, err := entry.Open()
	if err != nil {
		return "", fmt.Errorf("open publication entry %q: %w", entry.Name, err)
	}
	defer file.Close()
	hash := sha256.New()
	written, err := copyContext(ctx, hash, io.LimitReader(file, expectedSize+1))
	if err != nil {
		return "", fmt.Errorf("read publication entry %q: %w", entry.Name, err)
	}
	if written != expectedSize {
		return "", fmt.Errorf("publication entry %q size mismatch", entry.Name)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validatePackagePath(value string) error {
	if value == "" || strings.Contains(value, "\\") || path.IsAbs(value) || path.Clean(value) != value || strings.HasPrefix(value, "../") {
		return fmt.Errorf("unsafe publication package path %q", value)
	}
	return nil
}
