package service

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
)

const (
	Name   = "WorkloadInspector"
	Prefix = "INSP"
)

func StartNATSMicro(nc *nats.Conn, insp Inspector) error {
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
		microLogHandler(insp, ping),
		micro.WithEndpointSubject(fmt.Sprintf("%s.PING", Prefix)),
		micro.WithEndpointMetadata(map[string]string{
			"request": "",
		}),
	)
	if err != nil {
		return fmt.Errorf("error adding PING endpoint: %s", err)
	}

	err = svc.AddEndpoint(
		"ENV",
		microLogHandler(insp, getEnvironment),
		micro.WithEndpointSubject(fmt.Sprintf("%s.ENV", Prefix)),
		micro.WithEndpointMetadata(map[string]string{
			"request": "",
		}),
	)
	if err != nil {
		return fmt.Errorf("error adding ENV endpoint: %s", err)
	}

	err = svc.AddEndpoint(
		"RUN",
		microLogHandler(insp, runCommand),
		micro.WithEndpointSubject(fmt.Sprintf("%s.RUN", Prefix)),
		micro.WithEndpointMetadata(map[string]string{
			"request": `{"command": "string"}`,
		}),
	)
	if err != nil {
		return fmt.Errorf("error adding RUN endpoint: %s", err)
	}

	log.Printf("nats micro service started")
	return nil
}

func microLogHandler(svc Inspector, fn func(r micro.Request, svc Inspector)) micro.Handler {
	return micro.HandlerFunc(func(r micro.Request) {
		log.Printf("%s received request\n", r.Subject())
		fn(r, svc)
	})
}

func ping(r micro.Request, svc Inspector) {
	err := r.Respond([]byte("PONG"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ping response error: %s\n", err)
	}
}

func getEnvironment(r micro.Request, svc Inspector) {
	environ := svc.GetEnvironment()
	err := r.RespondJSON(environ)
	if err != nil {
		fmt.Fprintf(os.Stderr, "environment response error: %s\n", err)
	}
}

func runCommand(r micro.Request, svc Inspector) {
	var req RunCommandRequest
	err := json.Unmarshal(r.Data(), &req)
	if err != nil {
		err := fmt.Errorf("run request error: %s", err)
		fmt.Fprint(os.Stderr, err.Error())
		r.Error("100", err.Error(), nil)
		return
	}

	if req.Command == "" {
		err := fmt.Errorf("run request error: command is required")
		fmt.Fprintln(os.Stderr, err.Error())
		r.Error("100", err.Error(), nil)
		return
	}

	response, err := svc.RunCommand(req.Command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run error: %s\n", err)
		r.Error("100", err.Error(), nil)
		return
	}

	err = r.RespondJSON(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run response error: %s\n", err)
	}
}
