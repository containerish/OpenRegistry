FROM golang:alpine as build
LABEL org.opencontainers.image.source = "https://github.com/containerish/OpenRegistry"

WORKDIR /root/openregistry

COPY Makefile .
RUN apk add gcc make git curl ca-certificates && make mod-fix

COPY . .
RUN go mod download && CGO_ENABLED=0 go build -o openregistry -ldflags="-w -s" main.go

FROM alpine:latest

COPY --from=build /root/openregistry/openregistry .
#COPY ./config.yaml .
EXPOSE 5000
CMD ["./openregistry"]
