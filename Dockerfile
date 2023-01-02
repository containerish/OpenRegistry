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
WORKDIR /home/runner

ARG USER="runner"
ARG GROUP=${USER}
ARG UID=1001
ARG GID=${UID}
ARG HOME_DIR="/home/runner"

RUN addgroup --system ${GROUP} --gid ${GID} \
    && adduser ${USER} --uid ${UID} -G ${GROUP} --system --home ${HOME_DIR} --shell /bin/bash
COPY --chown=${USER}:${GROUP} --from=builder /root/openregistry/openregistry /bin/openregistry

USER ${USER}

EXPOSE 5000
ENTRYPOINT ["openregistry"]
