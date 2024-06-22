package hlds

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/rs/zerolog/log"
)

type Pool struct {
	docker     *docker.Client
	maxServers int
	servers    map[ServerID]server

	portsMutex sync.Mutex
	ports      []portAlloc // true = in use, false = free
}

type portAlloc struct {
	port  uint16
	inUse bool
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
		ports:      makePorts(minPort, maxServers),
	}, nil
}

func makePorts(minPort uint16, maxServers int) []portAlloc {
	var ret = make([]portAlloc, maxServers)

	for i := range ret {
		ret[i].port = uint16(i) + minPort
	}

	return ret
}

func (pool *Pool) AddServer(ctx context.Context, cfg ServerConfig) (ServerID, error) {
	var zero ServerID

	port, err := pool.AllocPort()
	if err != nil {
		return zero, fmt.Errorf("unable to allocate port: %w", err)
	}
	name := fmt.Sprintf("hlds_%d", port)

	containerConfig := cfg.ContainerConfig(port)
	hostConfig, tempFiles, err := cfg.HostConfig()
	if err != nil {
		return zero, fmt.Errorf("unable to create host config: %w", err)
	}

	log.Info().Str("name", name).Msg("creating container")
	res, err := pool.docker.ContainerCreate(ctx, &containerConfig, &hostConfig, nil, nil, name)
	if err != nil {
		return zero, fmt.Errorf("unable to create container: %w", err)
	}
	id := ServerID(res.ID)

	log.Info().Str("name", name).Str("id", res.ID).Msg("starting container")
	if err := pool.docker.ContainerStart(ctx, res.ID, types.ContainerStartOptions{}); err != nil {
		pool.forceRemoveContainer(ctx, id)
		return zero, fmt.Errorf("unable to start container: %w", err)
	}

	if len(res.Warnings) > 0 {
		log.Warn().Strs("warnings", res.Warnings).Msg("")
	}

	now := time.Now()
	if _, ok := pool.servers[id]; ok {
		return zero, fmt.Errorf("duplicate server id: %s", id)
	}

	pool.servers[id] = server{
		id:        id,
		name:      name,
		startedAt: now,
		expiresAt: now.Add(cfg.lifetime),
		tempFiles: tempFiles,
	}

	return ServerID(res.ID), nil
}

func (pool *Pool) RemoveServer(ctx context.Context, id ServerID) error {
	server := pool.servers[id]
	log.Info().Str("id", id.String()).Str("name", server.name).Msg("removing server")

	pool.forceRemoveContainer(ctx, server.id)
	if err := server.Close(); err != nil {
		return fmt.Errorf("unable to close server: %w", err)
	}

	delete(pool.servers, id)

	return nil
}

func (pool *Pool) forceRemoveContainer(ctx context.Context, id ServerID) {
	log.Info().Str("id", id.String()).Msg("removing container")
	if err := pool.docker.ContainerRemove(ctx, id.String(), types.ContainerRemoveOptions{
		Force: true,
	}); err != nil {
		log.Error().Err(err).Msg("unable to remove container")
	}
}

func (pool *Pool) AllocPort() (uint16, error) {
	pool.portsMutex.Lock()
	defer pool.portsMutex.Unlock()

	for i, v := range pool.ports {
		if v.inUse {
			continue
		}

		pool.ports[i].inUse = true
		return v.port, nil
	}

	return 0, errors.New("all ports are already allocated")
}

func (pool *Pool) FreePort(port uint16) {
	pool.portsMutex.Lock()
	defer pool.portsMutex.Unlock()

	for i, v := range pool.ports {
		if v.port != port {
			continue
		}

		pool.ports[i].inUse = false
		return
	}
}

func (pool *Pool) Run(ctx context.Context) error {
	timer := time.NewTicker(5 * time.Second)
	defer timer.Stop()
loop:
	for {
		select {
		case <-timer.C:
			if err := pool.removeExpiredServers(ctx); err != nil {
				return fmt.Errorf("unable to remove expired servers: %w", err)
			}
		case <-ctx.Done():
			break loop
		}
	}

	// The context being already cancelled, we need a "new" one to allow Docker
	// to do its stuff.
	return pool.close(context.WithoutCancel(ctx))
}

func (pool *Pool) removeExpiredServers(ctx context.Context) error {
	now := time.Now()
	var errs []error

	for _, v := range pool.servers {
		if now.Before(v.expiresAt) {
			continue
		}

		if err := pool.RemoveServer(ctx, v.id); err != nil {
			errs = append(errs, fmt.Errorf("unable to remove expired server: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (pool *Pool) close(ctx context.Context) error {
	var errs = make([]error, 0, len(pool.servers))
	for id := range pool.servers {
		if err := pool.RemoveServer(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("unable to remove server %s: %w", id, err))
		}
	}

	return errors.Join(errs...)
}
