#!/bin/sh
podman run --rm -v "$PWD/.:/proj" -w /proj gotify/build:1.25.1-linux-amd64  sh -c 'GOPROXY=https://goproxy.cn go build -a -installsuffix cgo -ldflags "-w -s" -buildmode=plugin -o napcat.so /proj'
