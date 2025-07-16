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
	NatsUrl  string
	NatsNkey string
	NatsJwt  string
}

type Config struct {
	Workloads WorkloadsConfig
	HttpPort  string
}

func LoadConfig() (*Config, error) {
	// Workloads config
	natsUrl := os.Getenv("NEX_WORKLOAD_NATS_URL")
	if natsUrl == "" {
		return nil, fmt.Errorf("missing NEX_WORKLOAD_NATS_URL")
	}

	natsNkey := strings.TrimSpace(os.Getenv("NEX_WORKLOAD_NATS_NKEY"))
	if natsNkey == "" {
		return nil, fmt.Errorf("missing NEX_WORKLOAD_NATS_NKEY")
	}

	natsJwtB64 := os.Getenv("NEX_WORKLOAD_NATS_B64_JWT")
	if natsJwtB64 == "" {
		return nil, fmt.Errorf("missing NEX_WORKLOAD_NATS_B64_JWT")
	}

	natsJwtBytes, err := base64.StdEncoding.DecodeString(natsJwtB64)
	if err != nil {
		return nil, fmt.Errorf("NEX_WORKLOAD_NATS_B64_JWT is invalid base64: %s", err)
	}
	natsJwt := strings.TrimSpace(string(natsJwtBytes))

	// Inspector config
	httpPort := os.Getenv("INSPECTOR_HTTP_PORT")

	return &Config{
		Workloads: WorkloadsConfig{
			NatsUrl:  natsUrl,
			NatsNkey: natsNkey,
			NatsJwt:  natsJwt,
		},
		HttpPort: httpPort,
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
