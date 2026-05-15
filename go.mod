module github.com/synadia-labs/workloads-demo

go 1.25.0

require (
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats.go v1.50.0
	github.com/nats-io/nkeys v0.4.15
	github.com/stretchr/testify v1.10.0
	github.com/synadia-io/orbit.go/microext v0.0.0-00010101000000-000000000000
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/nats-io/nats.go => github.com/joeriddles/nats.go v1.38.1-0.20260514222359-6eefbae4820b
	github.com/synadia-io/orbit.go/microext => github.com/joeriddles/orbit.go/microext v0.0.0-20260515185817-1cf75f3ba19b
)
