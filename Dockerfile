# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /inspector

FROM alpine:latest AS run
RUN apk add --no-cache ca-certificates
COPY --from=build --chown=appuser:appgroup /inspector /inspector
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
ENTRYPOINT ["/inspector"]
