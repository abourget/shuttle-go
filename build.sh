#!/bin/bash -xe

#CGO_ENABLED=0 
GOOS=linux GOARCH=amd64 go build -v -o shuttle-go
