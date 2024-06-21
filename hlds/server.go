package hlds

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

type ServerID string

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
	valveAddonsDirPath string

	maxPlayers int      // 2-32, we don't want to run singleplayer servers.
	mapCycle   []string // first entry as startup map
	cvars      CVars    // ends up in instance.cfg called by server.cfg
}

type serverState int

const (
	serverStateInvalid serverState = iota
	serverStateInitializing
	serverStateIdle
	serverStateShuttingDown
	serverStateDead
)

type server struct {
	id        ServerID
	state     serverState
	startedAt time.Time
	expiresAt time.Time
}

func NewServerConfig( // see type ServerConfig
	valveAddonsDirPath string,
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

	// Ensure we're not escaping our rudimentary chroot, relies on Abs calling Clean.
	absValveAddonsDirPath, err := filepath.Abs(valveAddonsDirPath)
	if err != nil {
		return zero, fmt.Errorf("unable to obtain absolute path for valveAddonsDirPath: %w", err)
	}

	if !strings.HasPrefix(absValveAddonsDirPath, UserContentDir+"/") {
		return zero, errors.New("valveAddonsDirPath outside of UserContentDir")
	}

	return ServerConfig{
		valveAddonsDirPath: absValveAddonsDirPath,
		maxPlayers:         maxPlayers,
		mapCycle:           mapCycle,
		cvars:              cvars,
	}, nil
}
