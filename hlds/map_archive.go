package hlds

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/rs/zerolog/log"
)

var MissingBSPErr = errors.New("no .bsp file in archive")
var MultipleBSPErr = errors.New("multiple .bsp files found in archive")
var InvalidPathErr = errors.New("archive contains paths with non-unicode characters")
var UnknownArchiveErr = errors.New("archive is not in a format we can handle")

type MapArchive struct {
	fs       fs.FS
	fsCloser func() error
	mapping  map[string]string // path in archive => path when extracting
}

type fsType int

const (
	fsTypeInvalid = iota
	fsTypeZIP
	fsType7z
	fsTypeRAR
)

// Will return fsTypeInvalid with no error if the format is unknown.
func detectArchiveType(path string) (fsType, error) {
	f, err := os.Open(path)
	if err != nil {
		return fsTypeInvalid, fmt.Errorf("unable to open archive: %w", err)
	}
	defer f.Close()

	var buf = make([]byte, 7)
	if _, err := f.Read(buf); err != nil {
		return fsTypeInvalid, fmt.Errorf("unable to read file header: %w", err)
	}

	switch {
	case bytes.Equal(buf[:6], []byte{0x37, 0x7a, 0xbc, 0xaf, 0x27, 0x1c}):
		return fsType7z, nil
	case bytes.Equal(buf[:4], []byte{0x50, 0x4b, 0x03, 0x04}):
		return fsTypeZIP, nil
	case bytes.Equal(buf[:7], []byte{0x52, 0x61, 0x72, 0x21, 0x1a, 0x07, 0x00}):
		return fsTypeRAR, nil
	}

	log.Debug().Hex("header", buf).Msg("unable to find correct file header")

	return fsTypeInvalid, nil
}

func archiveFSFactory(path string) (fs.FS, func() error, error) {
	typ, err := detectArchiveType(path)
	if err != nil {
		return nil, nil, err
	}

	switch typ {
	case fsTypeZIP:
		zip, err := zip.OpenReader(path)
		if err == nil {
			return zip, zip.Close, nil
		}

		return nil, nil, err
	case fsType7z:
		szip, err := sevenzip.OpenReader(path)
		if err == nil {
			return szip, szip.Close, nil
		}
		return nil, nil, err
	}

	return nil, nil, UnknownArchiveErr
}

// Please don't upload zip bombs to TWHL.
func ReadMapArchiveFromFile(path string) (*MapArchive, error) {
	fs, closer, err := archiveFSFactory(path)
	if err != nil {
		return nil, fmt.Errorf("unable to open map archive for reading: %w", err)
	}

	mapping, err := remapArchive(fs)
	if err != nil {
		return nil, fmt.Errorf("unable to remap archive paths: %w", err)
	}

	mapping, err = sanitizeMapping(mapping)
	if err != nil {
		return nil, fmt.Errorf("mapping sanitizing failed: %w", err)
	}
	log.Debug().Interface("mapping", mapping).Msg("")

	return &MapArchive{
		fs:       fs,
		fsCloser: closer,
		mapping:  mapping,
	}, nil
}

func (ma MapArchive) MapName() string {
	for _, v := range ma.mapping {
		if filepath.Dir(v) == "maps" && filepath.Ext(v) == ".bsp" {
			return strings.TrimSuffix(filepath.Base(v), ".bsp")
		}
	}

	panic("unreachable, there should definitely be a map available at this point")
}

func (ma MapArchive) Extract(dstBaseDir string) (int64, error) {
	log.Info().Str("dst", dstBaseDir).Msg("Extracting archive to disk.")

	var (
		total          int64
		extractedNames = make([]string, 0, len(ma.mapping))
		mapName        string
	)

	for srcName, dstName := range ma.mapping {
		written, err := ma.extractFile(srcName, filepath.Join(dstBaseDir, dstName))
		total += written

		extractedNames = append(extractedNames, dstName)
		if filepath.Ext(dstName) == ".bsp" {
			mapName = strings.TrimSuffix(dstName, ".bsp")
		}

		if err != nil {
			return total, fmt.Errorf("unable to extract from source '%s' to destination '%s': %w", srcName, dstName, err)
		}
	}

	resPath := filepath.Join(dstBaseDir, mapName+".res")
	if err := writeRESFile(resPath, extractedNames); err != nil {
		log.Error().Err(err).Str("path", resPath).Msg("unable to write RES file")
	}

	log.Info().Str("dst", dstBaseDir).Int64("uncompressed", total).Msg("Archive extracted.")

	return total, nil
}

func writeRESFile(path string, names []string) error {
	log.Debug().Str("path", path).Strs("names", names).Msg("writing RES file")

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("unable to open res file for writing: %w", err)
	}

	for _, v := range names {
		if filepath.Ext(v) == ".bsp" {
			continue
		}

		fmt.Fprintln(f, v)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("unable to finish writing to RES file: %w", err)
	}

	return nil
}

func (ma MapArchive) extractFile(srcName, dstPath string) (int64, error) {
	srcFile, err := ma.fs.Open(srcName)
	if err != nil {
		return 0, fmt.Errorf("unable to open source file '%s' in archive: %w", srcFile, err)
	}
	defer srcFile.Close()

	var dstDir = filepath.Dir(dstPath)
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return 0, fmt.Errorf("unable to create dir '%s': %w", dstDir, err)
		}
	}

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("unable to create file: %w", err)
	}

	written, err := io.Copy(dstFile, srcFile)
	if err != nil {
		return written, fmt.Errorf("unable to write to file: %w", err)
	}

	if err := dstFile.Close(); err != nil {
		return written, fmt.Errorf("unable to finish writing to dst: %w", err)
	}

	return written, nil
}

func (ma *MapArchive) Close() error {
	return ma.fsCloser()
}

// Get a usable tree out of random archives. ie. put bsp in maps/ even if they're
// at the root of the archive.
func remapArchive(archive fs.FS) (map[string]string, error) {
	var files []string

	if err := fs.WalkDir(archive, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error sent to WalkDir callback: %w", err)
		}

		if isPathGarbage(path) {
			log.Debug().Str("path", path).Msg("skipping garbage")
			return nil
		}

		files = append(files, filepath.Clean(path))
		return nil
	}); err != nil {
		if errors.Is(err, fs.ErrInvalid) {
			return nil, InvalidPathErr
		}

		return nil, fmt.Errorf("unable to walk through archive: %w", err)
	}

	log.Debug().Strs("files", files).Msg("")

	return generateMapping(files)
}

func isPathGarbage(path string) bool {
	switch {
	case strings.HasPrefix(path, "__MACOSX/"),
		filepath.Base(path) == ".DS_Store":
		return true
	}

	return false
}

func generateMapping(files []string) (map[string]string, error) {
	bspSrcPath, err := findBSPPath(files)
	if err != nil {
		return nil, fmt.Errorf("unable to find BSP: %w", err)
	}

	var (
		mapping = make(map[string]string, 1)
		// Consider the dir where we found the BSP to be the maps dir and build
		// the hierarchy from there (or rather: from its parent).
		mapsDir = filepath.Dir(bspSrcPath)
		baseDir = filepath.Dir(mapsDir)
	)

	// Lone BSP at the root of the archive, no other file is expected to be
	// usable or in the right path in this archive. Bail.
	if !strings.ContainsRune(bspSrcPath, '/') {
		log.Info().Str("bsp", bspSrcPath).Msg("Found BSP at the archive's root.")
		mapping[bspSrcPath] = filepath.Join("maps", bspSrcPath)
		return mapping, nil
	}

	// Someone caring put a lone BSP and maybe a readme in a subdirectory to
	// avoid zip bombing your cwd. Assume a lone BSP and bail.
	if filepath.Base(mapsDir) != "maps" {
		mapping[bspSrcPath] = filepath.Join("maps", filepath.Base(bspSrcPath))
		log.Warn().Str("bsp", bspSrcPath).Msg("Found BSP in a weird path.")
		return mapping, nil
	}

	log.Debug().Str("bsp", bspSrcPath).Str("base", baseDir).Msg("Found a proper hierarchy.")
	mapping[bspSrcPath] = filepath.Join("maps", bspSrcPath)
	return remapArchiveFromBaseDir(files, baseDir)
}

func remapArchiveFromBaseDir(files []string, baseDir string) (map[string]string, error) {
	var ret = make(map[string]string, len(files))

	for _, src := range files {
		if !archivePathHasPrefix(src, baseDir) {
			log.Debug().Str("src", src).
				Str("baseDir", baseDir).
				Msg("skipping file outside of found prefix")
			continue
		}

		ret[src] = archivePathTrimPrefix(src, baseDir)
	}

	return ret, nil
}

func archivePathTrimPrefix(path, prefix string) string {
	if prefix == "" {
		return path
	}

	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	if prefix == "./" {
		return path
	}

	return strings.TrimPrefix(path, prefix)
}

// Prefix as in "does this path starts with the given _path_", not _string_.
func archivePathHasPrefix(path, prefix string) bool {
	if prefix == "" {
		return true
	}

	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	if prefix == "./" {
		return true
	}

	return strings.HasPrefix(path, prefix)
}

func sanitizeMapping(mapping map[string]string) (map[string]string, error) {
	var (
		ret      = make(map[string]string, len(mapping))
		foundBSP bool
	)

	for src, dst := range mapping {
		if !isMappingDestValid(dst) {
			log.Debug().Str("src", src).Str("dst", dst).Msg("discarding invalid path")
			continue
		}

		ret[src] = dst

		foundBSP = foundBSP || filepath.Ext(dst) == ".bsp"
	}

	// Since we're removing paths, re-check if we have a BSP, just in case we
	// did something stupid.
	if !foundBSP {
		return nil, MissingBSPErr
	}

	return ret, nil
}

func isMappingDestValid(dst string) bool {
	var (
		dir = filepath.Dir(dst)
		ext = filepath.Ext(dst)
	)

	switch {
	case dir == "." && ext == ".wad",
		dir == "gfx/env" && ext == ".tga",
		dir == "maps" && (ext == ".bsp" || ext == ".cfg"), // ignore .res, we generate our own
		dir == "overviews" && (ext == ".tga" || ext == ".bmp" || ext == ".txt"),
		archivePathHasPrefix(dst, "sprites") && ext == ".spr",
		archivePathHasPrefix(dst, "sound") && ext == ".wav",
		archivePathHasPrefix(dst, "models") && ext == ".mdl":
		return true
	}

	return false
}

func findBSPPath(paths []string) (string, error) {
	var ret string

	for _, path := range paths {
		if strings.Contains(path, "..") {
			return "", fmt.Errorf("archive contains invalid paths: %s", path)
		}

		if strings.HasSuffix(path, ".bsp") {
			if ret != "" {
				return "", MultipleBSPErr
			}
			ret = path
		}
	}

	if ret == "" {
		return "", MissingBSPErr
	}

	return ret, nil
}
