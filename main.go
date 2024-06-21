package main

import (
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

	dispatch(pool)
	log.Info().Msg("HLDSBot shutdown complete. ")
}

func dispatch(pool *hlds.Pool) {
	var wg sync.WaitGroup
	done := make(chan struct{})
	go pool.Run(&wg, done)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	log.Info().Str("signal", sig.String()).Msg("Signal received.")

	close(done)
	log.Info().Msg("Shutting down.")
	wg.Wait()
}
