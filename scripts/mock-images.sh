#!/bin/bash

docker pull ubuntu
docker pull alpine 
docker pull postgres
docker pull httpd
docker pull nginx
docker pull busybox
docker pull golang
docker pull traefik

docker tag ubuntu 0.0.0.0:5000/johndoe/ubuntu
docker tag alpine 0.0.0.0:5000/johndoe/alpine
docker tag postgres 0.0.0.0:5000/johndoe/postgres
docker tag httpd 0.0.0.0:5000/johndoe/httpd
docker tag ubuntu 0.0.0.0:5000/johndoe/ubuntu:beta
docker tag alpine 0.0.0.0:5000/johndoe/alpine:beta
docker tag postgres 0.0.0.0:5000/johndoe/postgres:beta
docker tag httpd 0.0.0.0:5000/johndoe/httpd:beta
docker tag ubuntu 0.0.0.0:5000/johndoe/ubuntu:alpha
docker tag alpine 0.0.0.0:5000/johndoe/alpine:alpha
docker tag postgres 0.0.0.0:5000/johndoe/postgres:alpha
docker tag httpd 0.0.0.0:5000/johndoe/httpd:alpha
docker tag ubuntu 0.0.0.0:5000/johndoe/ubuntu:theta
docker tag alpine 0.0.0.0:5000/johndoe/alpine:theta
docker tag postgres 0.0.0.0:5000/johndoe/postgres:theta
docker tag httpd 0.0.0.0:5000/johndoe/httpd:theta

docker tag nginx 0.0.0.0:5000/chucknorris/ubuntu
docker tag busybox 0.0.0.0:5000/chucknorris/busybox
docker tag golang 0.0.0.0:5000/chucknorris/go
docker tag traefik 0.0.0.0:5000/chucknorris/traefik
docker tag nginx 0.0.0.0:5000/chucknorris/ubuntu:gamma
docker tag busybox 0.0.0.0:5000/chucknorris/busybox:gamma
docker tag golang 0.0.0.0:5000/chucknorris/go:gamma
docker tag traefik 0.0.0.0:5000/chucknorris/traefik:gamma
docker tag nginx 0.0.0.0:5000/chucknorris/ubuntu:hulk
docker tag busybox 0.0.0.0:5000/chucknorris/busybox:hulk
docker tag golang 0.0.0.0:5000/chucknorris/go:hulk
docker tag traefik 0.0.0.0:5000/chucknorris/traefik:hulk
docker tag nginx 0.0.0.0:5000/chucknorris/ubuntu:nimrod
docker tag busybox 0.0.0.0:5000/chucknorris/busybox:nimrod
docker tag golang 0.0.0.0:5000/chucknorris/go:nimrod
docker tag traefik 0.0.0.0:5000/chucknorris/traefik:nimrod

echo "Qwerty@123" | docker login 0.0.0.0:5000 --username johndoe --password-stdin
docker push 0.0.0.0:5000/johndoe/ubuntu
docker push 0.0.0.0:5000/johndoe/alpine
docker push 0.0.0.0:5000/johndoe/postgres
docker push 0.0.0.0:5000/johndoe/httpd
docker push 0.0.0.0:5000/johndoe/ubuntu:beta
docker push 0.0.0.0:5000/johndoe/alpine:beta
docker push 0.0.0.0:5000/johndoe/postgres:beta
docker push 0.0.0.0:5000/johndoe/httpd:beta
docker push 0.0.0.0:5000/johndoe/ubuntu:alpha
docker push 0.0.0.0:5000/johndoe/alpine:alpha
docker push 0.0.0.0:5000/johndoe/postgres:alpha
docker push 0.0.0.0:5000/johndoe/httpd:alpha
docker push 0.0.0.0:5000/johndoe/ubuntu:theta
docker push 0.0.0.0:5000/johndoe/alpine:theta
docker push 0.0.0.0:5000/johndoe/postgres:theta
docker push 0.0.0.0:5000/johndoe/httpd:theta

echo "Qwerty@123" | docker login 0.0.0.0:5000 --username chucknorris --password-stdin
docker push 0.0.0.0:5000/chucknorris/ubuntu
docker push 0.0.0.0:5000/chucknorris/busybox
docker push 0.0.0.0:5000/chucknorris/golang
docker push 0.0.0.0:5000/chucknorris/traefik
docker push 0.0.0.0:5000/chucknorris/ubuntu:gamma
docker push 0.0.0.0:5000/chucknorris/busybox:gamma
docker push 0.0.0.0:5000/chucknorris/go:gamma
docker push 0.0.0.0:5000/chucknorris/traefik:gamma
docker push 0.0.0.0:5000/chucknorris/ubuntu:hulk
docker push 0.0.0.0:5000/chucknorris/busybox:hulk
docker push 0.0.0.0:5000/chucknorris/go:hulk
docker push 0.0.0.0:5000/chucknorris/traefik:hulk
docker push 0.0.0.0:5000/chucknorris/ubuntu:nimrod
docker push 0.0.0.0:5000/chucknorris/busybox:nimrod
docker push 0.0.0.0:5000/chucknorris/go:nimrod
docker push 0.0.0.0:5000/chucknorris/traefik:nimrod
