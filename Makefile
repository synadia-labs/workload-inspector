run:
	go run .

test:
	go test ./...

build:
	go build -o ./target/inspector \

docker:
	docker buildx build --platform linux/amd64 \
		--tag workload-inspector:latest \
		--tag ghcr.io/synadia-labs/workload-inspector:latest \
		--label "org.opencontainers.image.source=https://github.com/synadia-labs/workload-inspector" \
 		--label "org.opencontainers.image.description=A simple NATS micro service to demonstrate running a workload in Synadia Cloud Workloads." \
		.

# https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry
ghcr-login:
	echo $$CR_PAT | docker login ghcr.io -u joeriddles --password-stdin

ghcr-push:
	docker push ghcr.io/synadia-labs/workload-inspector:latest
