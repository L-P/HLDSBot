package main

import (
	"context"
	"hldsbot/hlds"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	docker "github.com/docker/docker/client"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	})
	log.Info().Msg("Starting HLDSBot.")

	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to obtain Docker client")
	}
	defer func() {
		if err := dockerClient.Close(); err != nil {
			log.Error().Err(err).Msg("unable to close Docker client")
		}
	}()

	pool, err := hlds.NewPool(dockerClient, 2, 27015)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to init hlds.Pool")
	}

	dispatch(context.Background(), pool)
	log.Info().Msg("HLDSBot shutdown complete. ")
}

// Run all background processes and cleanup everything as soon as one of them
// closes or we're signaled to exit.
func dispatch(ctx context.Context, pool *hlds.Pool) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wrap(ctx, cancel, pool.Run, &wg)
	wrap(ctx, cancel, func(ctx context.Context) error { return debug(ctx, pool) }, &wg)

	<-ctx.Done()

	log.Info().Err(ctx.Err()).Msg("Shutting down.")
	wg.Wait()
}

type proc func(context.Context) error

// Wrap a background call with all the inter-cancellation shenanigans.
// If the context is canceled or the proc terminates, the cancelation will be
// propagated to all other procs launched by this func.
func wrap(ctx context.Context, cancel func(), f proc, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := f(ctx); err != nil {
			log.Error().Err(err).Msg("proc closed unexpectedly")
		}
		cancel()
	}()
}
