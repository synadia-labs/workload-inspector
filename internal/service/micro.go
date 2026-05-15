package service

import (
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
	"github.com/synadia-io/orbit.go/microext/openapi"
)

const (
	Name   = "WorkloadInspector"
	Prefix = "INSP"

	version     = "0.0.2"
	description = "NATS micro service to inspect a NEX workload environment."
)

type (
	PingRequest  struct{}
	PingResponse string

	EnvRequest  struct{}
	EnvResponse map[string]string

	RunRequest struct {
		Command string `json:"command"`
	}
	RunResponse struct {
		Stdout string `json:"stdout"`
		Stderr string `json:"stderr"`
		Code   int    `json:"code"`
		Error  string `json:"error,omitempty"`
	}
)

func StartNATSMicro(nc *nats.Conn, insp Inspector) (micro.Service, error) {
	svc, err := micro.AddService(nc, micro.Config{
		Name:        Name,
		Description: description,
		Version:     version,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating nats micro service: %v", err)
	}

	api, err := openapi.NewAPI(nc, svc, openapi.APIConfig{
		Title:       Name,
		Description: description,
		Version:     version,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating nats micro api: %v", err)
	}

	err = openapi.Register(api, "PING", func(r openapi.TypedRequest[PingRequest]) (*PingResponse, error) {
		log.Printf("%s received request\n", r.Subject())
		pong := PingResponse("PING")
		return &pong, nil
	}, openapi.WithSubject(fmt.Sprintf("%s.PING", Prefix)))
	if err != nil {
		return nil, fmt.Errorf("error adding PING endpoint: %s", err)
	}

	err = openapi.Register(api, "ENV", func(r openapi.TypedRequest[EnvRequest]) (*EnvResponse, error) {
		log.Printf("%s received request\n", r.Subject())
		env := EnvResponse(insp.GetEnvironment())
		return &env, nil
	}, openapi.WithSubject(fmt.Sprintf("%s.ENV", Prefix)))
	if err != nil {
		return nil, fmt.Errorf("error adding ENV endpoint: %s", err)
	}

	err = openapi.Register(api, "RUN", func(r openapi.TypedRequest[RunRequest]) (*RunResponse, error) {
		log.Printf("%s received request\n", r.Subject())
		req := r.Body()
		if req.Command == "" {
			return nil, fmt.Errorf("command is required")
		}
		res, err := insp.RunCommand(req.Command)
		if err != nil {
			return nil, err
		}
		return &RunResponse{
			Stdout: res.Stdout,
			Stderr: res.Stderr,
			Code:   res.Code,
			Error:  res.Error,
		}, nil
	}, openapi.WithSubject(fmt.Sprintf("%s.RUN", Prefix)))
	if err != nil {
		return nil, fmt.Errorf("error adding RUN endpoint: %s", err)
	}

	log.Printf("nats micro service started")
	return svc, err
}
