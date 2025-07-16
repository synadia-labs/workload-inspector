package config

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	NexWorkloadNatsServersEnvVar = "NEX_WORKLOAD_NATS_SERVERS"
	NexWorkloadNatsNkeyEnvVar    = "NEX_WORKLOAD_NATS_NKEY"
	NexWorkloadNatsB64JwtEnvVar  = "NEX_WORKLOAD_NATS_B64_JWT"
	InspectorHttpPortEnvVar      = "INSPECTOR_HTTP_PORT"
	InspectorHttpAuthEnvVar      = "INSPECTOR_HTTP_AUTH"
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

type WorkloadsConfig struct {
	NatsServers string
	NatsNkey    string
	NatsJwt     string
}

type HttpConfig struct {
	Port    string
	UseAuth bool
}

type Config struct {
	Workloads WorkloadsConfig
	Http      *HttpConfig
}

func LoadConfig() (*Config, error) {
	// Workloads config
	natsServers := os.Getenv(NexWorkloadNatsServersEnvVar)
	if natsServers == "" {
		return nil, fmt.Errorf("missing %s", NexWorkloadNatsServersEnvVar)
	}

	natsNkey := strings.TrimSpace(os.Getenv(NexWorkloadNatsNkeyEnvVar))
	if natsNkey == "" {
		return nil, fmt.Errorf("missing %s", NexWorkloadNatsNkeyEnvVar)
	}

	natsJwtB64 := os.Getenv(NexWorkloadNatsB64JwtEnvVar)
	if natsJwtB64 == "" {
		return nil, fmt.Errorf("missing %s", NexWorkloadNatsB64JwtEnvVar)
	}

	natsJwtBytes, err := base64.StdEncoding.DecodeString(natsJwtB64)
	if err != nil {
		return nil, fmt.Errorf("%s is invalid base64: %s", NexWorkloadNatsB64JwtEnvVar, err)
	}
	natsJwt := strings.TrimSpace(string(natsJwtBytes))

	// Inspector config
	httpPort := os.Getenv(InspectorHttpPortEnvVar)
	httpAuth := os.Getenv(InspectorHttpAuthEnvVar)

	return &Config{
		Workloads: WorkloadsConfig{
			NatsServers: natsServers,
			NatsNkey:    natsNkey,
			NatsJwt:     natsJwt,
		},
		Http: &HttpConfig{
			Port:    httpPort,
			UseAuth: httpAuth == "true",
		},
	}, nil
}

func (c *Config) SaveCreds() error {
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
			"Jwt":  c.Workloads.NatsJwt,
			"Nkey": c.Workloads.NatsNkey,
		})
		if err != nil {
			log.Fatalf("error writing nats creds file: %s", err)
		}
		log.Printf("nats creds file written to %s\n", file.Name())
	}

	return nil
}
