package hlds

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/rs/zerolog/log"
)

const (
	// To ensure this API cannot be misused and mount arbitrary directories, only
	// directories created under UserContentDir will be allowed to be mounted in
	// hlds containers.
	UserContentDir  = "/var/tmp/hlds"
	HLDSDockerImage = "hlds:latest"
)

const (
	valveAddonMountDest = "/home/steam/hlds/valve_addon"
	instanceCfgDest     = "/home/steam/hlds/valve/instance.cfg"
	mapCycleDest        = "/home/steam/hlds/valve/mapcycle.txt"
)

type ServerID string

func (id ServerID) String() string {
	return string(id)
}

type CVars map[string]string

func NewCVars() CVars {
	return make(map[string]string)
}

func isValidCVarRune(r rune) bool {
	if r > unicode.MaxASCII {
		return false
	}

	// Disallow quotes and quote the strings ourselves.
	// There's no escaping in goldsrc cvars and there's a bug that leads to
	// poorly bounded reads if you attempt something like: echo "abcde
	return r != '"'
}

func isStringValidCVar(s string) bool {
	for _, r := range s {
		if !isValidCVarRune(r) {
			return false
		}
	}

	return true
}

func (cvars CVars) Write(w io.Writer) error {
	for k, v := range cvars {
		if !isStringValidCVar(k) {
			return fmt.Errorf("invalid key in cvar: '%s'", k)
		}
		if !isStringValidCVar(k) {
			return fmt.Errorf("invalid value in cvar '%s': '%s'", k, v)
		}

		if _, err := fmt.Fprintf(w, `"%s" "%s"`+"\n", k, v); err != nil {
			return fmt.Errorf("unable to format cvars: %w", err)
		}
	}

	return nil
}

type ServerConfig struct {
	// Add the contents of this directory to the base game. Since HLDS doesn't
	// honor the addons_folder setting it's behavior will only be emulated by
	// copying the contents of the given dir inside the valve/ directory.
	// THIS MEANS SCRIPTS AND BINARIES WILL BE COPIED IF PRESENT.
	// The contents of this directory MUST be sanitized beforehand.
	// Can be left empty, no custom content will be loaded.
	// CONTENTS ON THE HOST WILL BE DELETED WHEN THE SERVER CLOSES
	valveAddonDirPath string

	lifetime   time.Duration
	maxPlayers int      // 2-32, we don't want to run singleplayer servers.
	mapCycle   []string // first entry as startup map
	cvars      CVars    // ends up in instance.cfg called by server.cfg
}

type server struct {
	id        ServerID
	name      string
	port      uint16
	startedAt time.Time
	expiresAt time.Time

	tempFiles []string // files to remove after closing the server
	addonsDir string
}

func (s *server) Close() error {
	var errs = make([]error, 0, len(s.tempFiles))

	for _, path := range s.tempFiles {
		log.Debug().Str("path", path).Msg("removing file")
		if err := os.Remove(path); err != nil {
			errs = append(errs, fmt.Errorf("unable to remove temp file '%s': %w", path, err))
		}
	}

	if s.addonsDir != "" && strings.HasPrefix(s.addonsDir, UserContentDir) {
		log.Debug().Str("path", s.addonsDir).Msg("removing dir")
		if err := os.RemoveAll(s.addonsDir); err != nil {
			errs = append(errs, fmt.Errorf("unable to remove addons dir: %w", err))
		}
	}

	return errors.Join(errs...)
}

func NewServerConfig( // see type ServerConfig
	lifetime time.Duration,
	valveAddonDirPath string,
	maxPlayers int,
	mapCycle []string,
	cvars CVars,
) (ServerConfig, error) {
	var zero ServerConfig

	if maxPlayers < 1 || maxPlayers > 32 {
		return zero, errors.New("maxPlayers out of bounds")
	}

	if len(mapCycle) < 1 {
		return zero, errors.New("mapCycle must contain at least one entry")
	}

	if lifetime < time.Minute || lifetime > (24*time.Hour) {
		return zero, errors.New("server lifetime must be within ]1m;24h]")
	}
	cvars["mp_timeleft"] = strconv.Itoa(int(lifetime.Seconds()))

	absValveAddonDirPath, err := resolveAddonDirPath(valveAddonDirPath)
	if err != nil {
		return zero, fmt.Errorf("unable to resolve path to valve_addon dir: %w", err)
	}

	return ServerConfig{
		valveAddonDirPath: absValveAddonDirPath,
		maxPlayers:        maxPlayers,
		mapCycle:          mapCycle,
		cvars:             cvars,
		lifetime:          lifetime,
	}, nil
}

func (cfg ServerConfig) ContainerConfig(port uint16) container.Config {
	return container.Config{
		Cmd: []string{
			"-norestart", "-nohltv",
			"-port", strconv.Itoa(int(port)),
			"-maxplayers", "32",
			"+map", cfg.mapCycle[0],
		},
		Image: HLDSDockerImage,
	}
}

// Returns a list of temp files to remove once the server is to be deleted.
func (cfg ServerConfig) HostConfig() (container.HostConfig, []string, error) {
	mounts, tempFiles, err := cfg.writeConfigToDockerMounts()
	if err != nil {
		return container.HostConfig{}, nil, fmt.Errorf("unable to write server configuration: %w", err)
	}

	return container.HostConfig{
		NetworkMode: "host",
		AutoRemove:  true,
		Mounts:      mounts,
	}, tempFiles, nil
}

func writeCVarsToTempfile(cvars CVars) (string, error) {
	f, err := os.CreateTemp("", "cvars.*.cfg")
	if err != nil {
		return "", fmt.Errorf("unable to create temp file: %w", err)
	}

	if err := cvars.Write(f); err != nil {
		return "", fmt.Errorf("unable to write cvars to temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		return "", fmt.Errorf("unable to finish writing to temp file: %w", err)
	}

	return f.Name(), nil
}

func writeMapcycleToTempfile(mapCycle []string) (string, error) {
	f, err := os.CreateTemp("", "mapcycle.*.txt")
	if err != nil {
		return "", fmt.Errorf("unable to create temp file: %w", err)
	}

	for _, v := range mapCycle {
		fmt.Fprintln(f, v)
	}

	if err := f.Close(); err != nil {
		return "", fmt.Errorf("unable to finish writing to temp file: %w", err)
	}

	return f.Name(), nil
}

func (cfg ServerConfig) writeConfigToDockerMounts() ([]mount.Mount, []string, error) {
	// TODO listip.cfg,banned.cfg
	var ret = make([]mount.Mount, 0, 2)
	var tmpfiles = make([]string, 0, len(ret))

	instanceCfgSrc, err := writeCVarsToTempfile(cfg.cvars)
	tmpfiles = append(tmpfiles, instanceCfgSrc)
	if err != nil {
		return nil, tmpfiles, fmt.Errorf("unable to write instance configuration: %w", err)
	}
	ret = append(ret, mount.Mount{
		Type:     mount.TypeBind,
		Source:   instanceCfgSrc,
		Target:   instanceCfgDest,
		ReadOnly: true,
	})

	mapCycleSrc, err := writeMapcycleToTempfile(cfg.mapCycle)
	if err != nil {
		return nil, tmpfiles, fmt.Errorf("unable to write mapcycle: %w", err)
	}
	tmpfiles = append(tmpfiles, mapCycleSrc)
	ret = append(ret, mount.Mount{
		Type:     mount.TypeBind,
		Source:   mapCycleSrc,
		Target:   mapCycleDest,
		ReadOnly: true,
	})

	if cfg.valveAddonDirPath != "" {
		ret = append(ret, mount.Mount{
			Type:     mount.TypeBind,
			Source:   cfg.valveAddonDirPath,
			Target:   valveAddonMountDest,
			ReadOnly: true,
		})
	}

	return ret, tmpfiles, nil
}

// Ensures we're not escaping our rudimentary chroot, relies on Abs calling Clean.
func resolveAddonDirPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("unable to obtain absolute path for valveAddonDirPath: %w", err)
	}

	if !strings.HasPrefix(abs, UserContentDir+"/") {
		return "", errors.New("valveAddonDirPath outside of UserContentDir")
	}

	return abs, nil
}
