package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/google/shlex"
)

type RunResult struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Code   int    `json:"code"`
	Error  string `json:"error,omitempty"`
}

type Inspector interface {
	// pong
	Ping() string

	// get the environment variables for the host
	GetEnvironment() map[string]string

	// run a command on the host
	RunCommand(command string) (*RunResult, error)
}

func NewInspector() Inspector {
	return &inspector{}
}

type inspector struct {
}

func (i *inspector) Ping() string {
	return "PONG"
}

func (i *inspector) GetEnvironment() map[string]string {
	environ := map[string]string{}
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		key := parts[0]
		val := parts[1]
		environ[key] = val
	}
	return environ
}

func (i *inspector) RunCommand(command string) (*RunResult, error) {
	if command == "" {
		return &RunResult{}, nil
	}

	response, err := i.parseAndRun(command)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (i *inspector) parseAndRun(command string) (*RunResult, error) {
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

	return i.pipeCommands(commands)
}

// Run a series of piped commands
func (i *inspector) pipeCommands(commands []*exec.Cmd) (*RunResult, error) {
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
				return &RunResult{
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
				return &RunResult{
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
			return &RunResult{
				Stdout: stdoutBuf.String(),
				Stderr: stderr,
				Code:   cmd.ProcessState.ExitCode(),
				Error:  err.Error(),
			}, fmt.Errorf("error running command \"%s\": %s", strings.Join(cmd.Args, " "), err)
		}
	}

	return &RunResult{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
		Code:   commands[len(commands)-1].ProcessState.ExitCode(),
	}, nil
}
