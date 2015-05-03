#!/bin/bash
godep get
CGO_ENABLED=0 go build -a -ldflags -d
mv docker-volumes /docker-volumes
