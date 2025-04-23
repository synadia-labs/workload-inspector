package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
)

const (
	Name   = "WorkloadInspector"
	Prefix = "INSP"
)

func main() {
	// pre-flight checks
	natsUrl := os.Getenv("NEX_WORKLOAD_NATS_URL")
	if natsUrl == "" {
		log.Fatalf("missing NEX_WORKLOAD_NATS_URL")
	}

	natsNkey := os.Getenv("NEX_WORKLOAD_NATS_NKEY")
	if natsUrl == "" {
		log.Fatalf("missing NEX_WORKLOAD_NATS_NKEY")
	}

	natsJwtB64 := os.Getenv("NEX_WORKLOAD_NATS_B64_JWT")
	if natsJwtB64 == "" {
		log.Fatalf("missing NEX_WORKLOAD_NATS_B64_JWT")
	}

	natsJwtBytes, err := base64.StdEncoding.DecodeString(natsJwtB64)
	if err != nil {
		log.Fatalf("NEX_WORKLOAD_NATS_B64_JWT is invalid base64: %s", err)
	}
	natsJwt := string(natsJwtBytes)

	// connect to nats
	nc, err := nats.Connect(natsUrl, nats.UserJWTAndSeed(natsJwt, natsNkey), nats.Name(Name))
	if err != nil {
		log.Fatalf("error connecting to nats: %s", err)
	}
	defer nc.Close()

	// start service
	svc, err := micro.AddService(nc, micro.Config{
		Name:        Name,
		Description: "NATS micro service to inspect a NEX workload environment.",
		Version:     "0.0.1",
	})
	if err != nil {
		log.Fatalf("error creating nats micro service: %s", err)
	}
	defer svc.Stop()

	err = svc.AddEndpoint(
		"PING",
		micro.HandlerFunc(ping),
		micro.WithEndpointSubject(fmt.Sprintf("%s.PING", Prefix)),
		micro.WithEndpointMetadata(map[string]string{
			"request": "",
		}),
	)
	if err != nil {
		log.Fatalf("error adding PING endpoint: %s", err)
	}

	err = svc.AddEndpoint(
		"ENV",
		micro.HandlerFunc(getEnvironment),
		micro.WithEndpointSubject(fmt.Sprintf("%s.ENV", Prefix)),
		micro.WithEndpointMetadata(map[string]string{
			"request": "",
		}),
	)
	if err != nil {
		log.Fatalf("error adding ENV endpoint: %s", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("%s started\n", Name)
	<-ctx.Done()
	fmt.Printf("%s stopped", Name)
}

func ping(r micro.Request) {
	err := r.Respond([]byte("PONG"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ping response error: %s\n", err)
	}
}

func getEnvironment(r micro.Request) {
	environ := map[string]string{}
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		key := parts[0]
		val := parts[1]
		environ[key] = val
	}
	err := r.RespondJSON(environ)
	if err != nil {
		fmt.Fprintf(os.Stderr, "environment response error: %s\n", err)
	}
}
