package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"text/template"

	"github.com/google/shlex"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
)

const (
	Name   = "WorkloadInspector"
	Prefix = "INSP"
)

const credsTempl = `-----BEGIN NATS USER JWT-----
{{.Jwt}}
------END NATS USER JWT------

************************* IMPORTANT *************************
NKEY Seed printed below can be used to sign and prove identity.
NKEYs are sensitive and should be treated as secrets.

-----BEGIN USER NKEY SEED-----
{{.Nkey}}
------END USER NKEY SEED------

*************************************************************`

func main() {
	// pre-flight checks
	natsUrl := os.Getenv("NEX_WORKLOAD_NATS_URL")
	if natsUrl == "" {
		log.Fatalf("missing NEX_WORKLOAD_NATS_URL")
	}

	natsNkey := strings.TrimSpace(os.Getenv("NEX_WORKLOAD_NATS_NKEY"))
	if natsNkey == "" {
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
	natsJwt := strings.TrimSpace(string(natsJwtBytes))

	// save user creds to file if inside a container
	if os.Getenv("container") != "" {
		tmpl, err := template.New("creds").Parse(credsTempl)
		if err != nil {
			log.Fatalf("error parsing creds template: %s", err)
		}

		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("error getting user home directory: %s", err)
		}

		file, err := os.Create(filepath.Join(home, "creds.txt"))
		if err != nil {
			log.Fatalf("error creating nats creds file: %s", err)
		}

		{
			defer file.Close()
			err = tmpl.Execute(file, map[string]string{
				"Jwt":  natsJwt,
				"Nkey": natsNkey,
			})
			if err != nil {
				log.Fatalf("error writing nats creds file: %s", err)
			}
			log.Printf("nats creds file written to %s\n", file.Name())
		}
	}

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
		microLogHandler(ping),
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
		microLogHandler(getEnvironment),
		micro.WithEndpointSubject(fmt.Sprintf("%s.ENV", Prefix)),
		micro.WithEndpointMetadata(map[string]string{
			"request": "",
		}),
	)
	if err != nil {
		log.Fatalf("error adding ENV endpoint: %s", err)
	}

	err = svc.AddEndpoint(
		"RUN",
		microLogHandler(runCommand),
		micro.WithEndpointSubject(fmt.Sprintf("%s.RUN", Prefix)),
		micro.WithEndpointMetadata(map[string]string{
			"request": `{"command": "string"}`,
		}),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("%s started\n", Name)
	<-ctx.Done()
	log.Printf("%s stopped", Name)
}

func microLogHandler(fn func(r micro.Request)) micro.Handler {
	return micro.HandlerFunc(func(r micro.Request) {
		log.Printf("%s received request\n", r.Subject())
		fn(r)
	})
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

type RunCommandRequest struct {
	Command string `json:"command"`
}

type RunCommandResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Code   int    `json:"code"`
	Error  string `json:"error,omitempty"`
}

func runCommand(r micro.Request) {
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

	response, err := parseAndRun(req.Command)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run error: %s\n", err)
		var data []byte
		if response != nil {
			data, _ = json.Marshal(response)
		}
		r.Error("100", err.Error(), data)
		return
	}

	err = r.RespondJSON(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run response error: %s\n", err)
	}
}

func parseAndRun(command string) (*RunCommandResponse, error) {
	args, err := shlex.Split(command)
	if err != nil {
		return nil, fmt.Errorf("error parsing command \"%s\": %s", command, err)
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("no command provided")
	}

	// Split the command into piped commands
	commands := []*exec.Cmd{}
	for {
		if !slices.Contains(args, "|") {
			commands = append(commands, exec.Command(args[0], args[1:]...))
			break
		}

		idx := slices.IndexFunc(args, func(arg string) bool {
			return arg == "|"
		})
		if idx != -1 {
			commands = append(commands, exec.Command(args[0], args[1:idx]...))
			args = args[idx+1:]
		}
	}

	return pipeCommands(commands)
}

// Run a series of piped commands
func pipeCommands(commands []*exec.Cmd) (*RunCommandResponse, error) {
	var stderrBuf bytes.Buffer
	var stdoutBuf bytes.Buffer

	readers := []*os.File{}
	writers := []*os.File{}

	for i, cmd := range commands {
		if i > 0 {
			// redirect stdin from the previous command's stdout
			cmd.Stdin = readers[i-1]
		}

		r, w, err := os.Pipe()
		if err != nil {
			return nil, fmt.Errorf("error creating pipe: %s", err)
		}
		readers = append(readers, r)
		writers = append(writers, w)

		// always redirect stderr to the buffer
		cmd.Stderr = &stderrBuf

		// if the last command, redirect stdout to the buffer, otherwise redirect to the next command
		if i == len(commands)-1 {
			cmd.Stdout = &stdoutBuf
		} else {
			cmd.Stdout = w
		}

		err = cmd.Start()
		if err != nil {
			return nil, fmt.Errorf("error starting command \"%s\": %s", strings.Join(cmd.Args, " "), err)
		}
	}

	for i, cmd := range commands {
		if i > 0 {
			err := readers[i-1].Close()
			if err != nil {
				stderr := stderrBuf.String()
				fmt.Fprintf(os.Stderr, "%s\n", stderr)
				return &RunCommandResponse{
					Stdout: stdoutBuf.String(),
					Stderr: stderr,
					Code:   cmd.ProcessState.ExitCode(),
					Error:  err.Error(),
				}, fmt.Errorf("error closing reader: %s", err)
			}
			err = writers[i-1].Close()
			if err != nil {
				stderr := stderrBuf.String()
				fmt.Fprintf(os.Stderr, "%s\n", stderr)
				return &RunCommandResponse{
					Stdout: stdoutBuf.String(),
					Stderr: stderr,
					Code:   cmd.ProcessState.ExitCode(),
					Error:  err.Error(),
				}, fmt.Errorf("error closing writer: %s", err)
			}
		}
		err := cmd.Wait()
		if err != nil {
			stderr := stderrBuf.String()
			fmt.Fprintf(os.Stderr, "%s\n", stderr)
			return &RunCommandResponse{
				Stdout: stdoutBuf.String(),
				Stderr: stderr,
				Code:   cmd.ProcessState.ExitCode(),
				Error:  err.Error(),
			}, fmt.Errorf("error running command \"%s\": %s", strings.Join(cmd.Args, " "), err)
		}
	}

	return &RunCommandResponse{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
		Code:   commands[len(commands)-1].ProcessState.ExitCode(),
	}, nil
}
