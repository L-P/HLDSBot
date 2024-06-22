package hlds

import (
	"context"
	"errors"
	"fmt"
	"math"

	docker "github.com/docker/docker/client"
)

// To ensure this API cannot be misused and mount arbitrary directories, only
// directories created under UserContentDir will be allowed to be mounted in
// hlds containers.
const UserContentDir = "/var/tmp/hlds"

type Pool struct {
	docker     *docker.Client
	maxServers int
	servers    map[ServerID]server
	ports      []bool // true = in use, false = free
}

func NewPool(
	dockerClient *docker.Client,
	maxServers int,
	minPort uint16,
) (*Pool, error) {
	// Let the OS throw when a bad port is bound, only do basic checks.
	if maxServers < 1 || maxServers >= math.MaxUint16 {
		return nil, fmt.Errorf("maxServers out of bounds: %d", maxServers)
	}

	if int(minPort)+maxServers > math.MaxUint16 {
		return nil, errors.New("port overflow, minPort+maxServers > maxPort")
	}

	return &Pool{
		docker:     dockerClient,
		maxServers: maxServers,
		servers:    make(map[ServerID]server, maxServers),
		ports:      make([]bool, maxServers),
	}, nil
}

func (pool *Pool) AddServer(cfg ServerConfig) (ServerID, error) {
	return ServerID(""), errors.New("not implemented")
}

func (pool *Pool) Run(ctx context.Context) error {
	// TODO health checks, container maintenance, Whatever.
	<-ctx.Done()

	return nil
}
