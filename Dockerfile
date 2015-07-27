FROM debian:jessie
ENV DOCKERVOLUMES_VERSION 1.1.2
RUN apt-get update && apt-get install -y curl ca-certificates --no-install-recommends \
  && curl -SLf https://github.com/cpuguy83/docker-volumes/releases/download/v${DOCKERVOLUMES_VERSION}/docker-volumes-linux-amd64 > /usr/bin/docker-volumes \
  && chmod +x /usr/bin/docker-volumes \
  && apt-get remove --purge curl ca-certificates -y \
  && rm -rf /var/lib/apt/lists
ENTRYPOINT ["/usr/bin/docker-volumes"]
