FROM golang:1.3-cross
ADD . /go/src/github.com/cpuguy83/docker-volumes
WORKDIR /go/src/github.com/cpuguy83/docker-volumes
ENV GOOS linux
ENV GOARCH amd64
RUN go get
ENTRYPOINT ["/go/src/github.com/cpuguy83/docker-volumes/make.sh"]
