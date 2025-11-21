package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/synadia-labs/workloads-demo/internal/config"
	"github.com/synadia-labs/workloads-demo/internal/service"
)

const name = service.Name

func main() {
	// dump env vars
	for _, env := range os.Environ() {
		log.Printf("%s=%s", env, os.Getenv(env))
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("error loading config: %s", err)
	}

	// save user creds to file if inside a container
	if os.Getenv("container") != "" {
		err := cfg.SaveCreds()
		if err != nil {
			log.Fatalf("error saving nats creds: %s", err)
		}
	}

	// connect to nats
	nc, err := nats.Connect(cfg.Workloads.NatsServers, nats.UserJWTAndSeed(cfg.Workloads.NatsJwt, cfg.Workloads.NatsNkey), nats.Name(name))
	if err != nil {
		log.Fatalf("error connecting to nats: %s", err)
	}
	defer nc.Close()

	// create service
	insp := service.NewInspector()

	// start micro service
	svc, err := service.StartNATSMicro(nc, insp)
	if err != nil {
		log.Fatalf("error starting micro service: %s", err)
	}
	defer svc.Stop()

	// start HTTP server
	if cfg.Http != nil {
		server := service.NewHTTPServer(cfg.Http, insp)
		go func() {
			err = server.Start()
			if err != nil {
				log.Fatalf("error starting HTTP server: %s", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("%s started\n", name)
	<-ctx.Done()
	log.Printf("%s stopped", name)
}
