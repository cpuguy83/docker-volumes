FROM golang:1.3-cross
ADD . /opt/dvm
WORKDIR /opt/dvm
ENV GOOS linux
ENV GOARCH amd64
ENTRYPOINT ["/opt/dvm/make.sh"]
