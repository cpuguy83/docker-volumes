FROM golang:1.4-cross
ADD . /go/src/github.com/cpuguy83/docker-volumes
WORKDIR /go/src/github.com/cpuguy83/docker-volumes
ENV GOOS linux
ENV GOARCH amd64
RUN go get github.com/tools/godep && godep get
CMD ["/go/src/github.com/cpuguy83/docker-volumes/make.sh"]
