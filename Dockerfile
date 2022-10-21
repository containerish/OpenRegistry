FROM golang:1.19-alpine as builder

ENV CGO_ENABLED=0
WORKDIR /root/openregistry

# Helps with caching
COPY go.mod go.sum ./
RUN go mod download

## Build the binary
COPY . .
RUN go build -o openregistry -ldflags="-w -s" -trimpath main.go

FROM alpine:latest
LABEL org.opencontainers.image.source = "https://github.com/containerish/OpenRegistry"
RUN adduser runner -D -S -h /home/runner
WORKDIR /home/runner

COPY --from=builder /root/openregistry/openregistry /bin/openregistry

USER runner
EXPOSE 5000
ENTRYPOINT ["openregistry"]
