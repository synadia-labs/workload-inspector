package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
	"github.com/synadia-labs/workloads-demo/internal/config"
	"github.com/synadia-labs/workloads-demo/internal/service"
)

const (
	Name   = "WorkloadInspector"
	Prefix = "INSP"
)

func main() {
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
	nc, err := nats.Connect(cfg.Workloads.NatsUrl, nats.UserJWTAndSeed(cfg.Workloads.NatsJwt, cfg.Workloads.NatsNkey), nats.Name(Name))
	if err != nil {
		log.Fatalf("error connecting to nats: %s", err)
	}
	defer nc.Close()

	// create service
	insp := service.NewInspector()

	// start micro service
	err = startMicroService(nc, insp)
	if err != nil {
		log.Fatalf("error starting micro service: %s", err)
	}

	// start HTTP server
	if cfg.HttpPort != "" {
		go func() {
			err = startHTTPServer(insp, cfg.HttpPort)
			if err != nil {
				log.Fatalf("error starting HTTP server: %s", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("%s started\n", Name)
	<-ctx.Done()
	log.Printf("%s stopped", Name)
}

// start HTTP server
func startHTTPServer(insp service.Inspector, port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(insp.Ping()))
	})

	mux.HandleFunc("GET /env", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		environ := insp.GetEnvironment()
		json.NewEncoder(w).Encode(environ)
	})

	mux.HandleFunc("POST /run", func(w http.ResponseWriter, r *http.Request) {
		var req RunCommandRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, fmt.Errorf(`expected request format is {"command": "string"}`).Error(), http.StatusBadRequest)
			return
		}
		response, err := insp.RunCommand(req.Command)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("http server started on %s", addr)
	return http.ListenAndServe(addr, mux)
}

// start micro service
func startMicroService(nc *nats.Conn, insp service.Inspector) error {
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

func microLogHandler(svc service.Inspector, fn func(r micro.Request, svc service.Inspector)) micro.Handler {
	return micro.HandlerFunc(func(r micro.Request) {
		log.Printf("%s received request\n", r.Subject())
		fn(r, svc)
	})
}

func ping(r micro.Request, svc service.Inspector) {
	err := r.Respond([]byte("PONG"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ping response error: %s\n", err)
	}
}

func getEnvironment(r micro.Request, svc service.Inspector) {
	environ := svc.GetEnvironment()
	err := r.RespondJSON(environ)
	if err != nil {
		fmt.Fprintf(os.Stderr, "environment response error: %s\n", err)
	}
}

type RunCommandRequest struct {
	Command string `json:"command"`
}

type RunCommandResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Code   int    `json:"code"`
	Error  string `json:"error,omitempty"`
}

func runCommand(r micro.Request, svc service.Inspector) {
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
