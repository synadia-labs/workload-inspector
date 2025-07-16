target "default" {
  context = "."
  dockerfile = "Dockerfile"
  platforms = ["linux/amd64", "linux/arm64"]
  tags = [
    "workload-inspector:latest",
    "ghcr.io/synadia-labs/workload-inspector:latest"
  ]
  annotations = [
    "org.opencontainers.image.source=https://github.com/synadia-labs/workload-inspector",
    "org.opencontainers.image.description=A simple NATS micro service to demonstrate running a workload in Synadia Cloud Workloads."
  ]
}
