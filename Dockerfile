FROM golang:1.3-cross
ADD . /opt/docker-volumes
WORKDIR /opt/docker-volumes
ENV GOOS linux
ENV GOARCH amd64
ENTRYPOINT ["/opt/docker-volumes/make.sh"]
