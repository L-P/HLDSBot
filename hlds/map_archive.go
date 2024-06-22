package hlds

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

type MapArchive struct {
	zip *zip.ReadCloser

	mapping map[string]string // path in zip => path when extracting
}

// Please don't upload zip bombs to TWHL.
func ReadMapArchiveFromFile(path string) (*MapArchive, error) {
	zip, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("unable to open map archive for reading: %w", err)
	}

	mapping, err := remapZIP(zip)
	if err != nil {
		return nil, fmt.Errorf("unable to remap zip paths: %w", err)
	}

	mapping, err = sanitizeMapping(mapping)
	if err != nil {
		return nil, fmt.Errorf("mapping sanitizing failed: %w", err)
	}
	log.Debug().Interface("mapping", mapping).Msg("")

	return &MapArchive{
		zip:     zip,
		mapping: mapping,
	}, nil
}

func (ma MapArchive) Extract(dstBaseDir string) (int64, error) {
	log.Info().Str("dst", dstBaseDir).Msg("Extracting archive to disk.")

	var total int64

	for srcName, dstName := range ma.mapping {
		written, err := ma.extractFile(srcName, filepath.Join(dstBaseDir, dstName))
		total += written

		if err != nil {
			return total, fmt.Errorf("unable to extract from source '%s' to destination '%s': %w", srcName, dstName, err)
		}
	}

	log.Info().Str("dst", dstBaseDir).Int64("uncompressed", total).Msg("Archive extracted.")

	return total, nil
}

func (ma MapArchive) extractFile(srcName, dstPath string) (int64, error) {
	srcFile, err := ma.zip.Open(srcName)
	if err != nil {
		return 0, fmt.Errorf("unable to open source file '%s' in zip: %w", srcFile, err)
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
	return ma.zip.Close()
}

// Get a usable tree out of random zips. ie. put bsp in maps/ even if they're
// at the root of the archive.
func remapZIP(zip *zip.ReadCloser) (map[string]string, error) {
	var files = make([]string, len(zip.File))
	for i := range zip.File {
		files[i] = filepath.Clean(zip.File[i].Name)
	}

	return remapArchive(files)
}

func remapArchive(files []string) (map[string]string, error) {
	var (
		bspSrcPath string
		mapping    = make(map[string]string, 1)
	)

	for _, path := range files {
		if strings.Contains(path, "..") {
			return nil, fmt.Errorf("archive contains invalid paths: %s", path)
		}

		if strings.HasSuffix(path, ".bsp") {
			if bspSrcPath != "" {
				return nil, fmt.Errorf("multiple .bsp files in archive, not deciding")
			}
			bspSrcPath = path
		}
	}
	mapping[bspSrcPath] = filepath.Join("maps", bspSrcPath)

	// Lone BSP at the root of the archive, no other file is expected to be
	// usable or in the right path in this ZIP. Bail.
	if !strings.ContainsRune(bspSrcPath, '/') {
		log.Info().Str("bsp", bspSrcPath).Msg("found BSP at the archive's root")
		return mapping, nil
	}

	// What? Assume a lone .bsp lost in the zip and bail.
	mapsDir := filepath.Dir(bspSrcPath)
	if filepath.Base(mapsDir) != "maps" {
		log.Warn().Str("bsp", bspSrcPath).Msg("found BSP in a weird path")
		return mapping, nil
	}

	baseDir := filepath.Dir(mapsDir)
	log.Info().Str("bsp", bspSrcPath).Str("base", baseDir).Msg("found a seemingly proper hierarchy")

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
	var ret = make(map[string]string, len(mapping))
	var foundBSP bool

	for src, dst := range mapping {
		if !isMappingDestValid(dst) {
			log.Debug().Str("src", src).Str("dst", dst).Msg("discarding invalid path")
			continue
		}

		ret[src] = dst

		foundBSP = foundBSP || filepath.Ext(dst) == ".bsp"
	}

	if !foundBSP {
		return nil, errors.New("found no BSP after sanitizing")
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
		dir == "maps" && (ext == ".bsp" || ext == ".res" || ext == ".cfg"),
		dir == "sprites" && ext == ".spr",
		dir == "sound" && ext == ".wav",
		dir == "models" && ext == ".mdl",
		dir == "overviews" && (ext == ".tga" || ext == ".bmp" || ext == ".txt"):
		return true
	}

	return false
}
