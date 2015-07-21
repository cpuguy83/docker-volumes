#!/bin/bash
godep get
go build -a -tags netgo -installsuffix netgo .
mv docker-volumes /docker-volumes
