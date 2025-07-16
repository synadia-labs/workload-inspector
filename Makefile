run:
	go run .

test:
	go test ./...

build:
	go build -o ./target/inspector \

docker:
	docker buildx bake -f ./docker-bake.hcl

compose:
	docker compose up

# https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry
ghcr-login:
	echo $$CR_PAT | docker login ghcr.io -u joeriddles --password-stdin

ghcr-push:
	docker push ghcr.io/synadia-labs/workload-inspector:latest
