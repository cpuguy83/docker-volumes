# Docker Volume Manager

This is a tool for managing your Docker volumes.

Basically, since volumes are not yet first-class citizens in Docker they can be
difficult to manage. Most people tend to have extra volumes laying around which
are not in use because they didn't get removed with the container they were used
with.

You can run this tool remotely just as you do with the Docker CLI.  It reads
DOCKER_HOST or you can specify a host using the same syntax as with Docker, with
`-H unix:///path/to/sock.sock` or `--host unix:///path/to/sock.sock`.
*This also works with TCP endpoints*

Use this to see all your volumes, inspect them, export them, or clean them up.

## Installation

You can use the provided Dockerfile which will compile a binary for you or build
yourself.

```bash
docker build -t docker-volumes git@github.com:cpuguy83/docker-volumes.git
docker run --name docker-volumes docker-volumes
docker cp docker-volumes:/opt/docker-volumes/docker-volumes ./
```

By default when compiling from the Dockerfile it will compile for linux/amd64.
You can customize this using environment variables as such:

```bash
docker run -d --name docker-volumes -e GOOS=darwin -e GOARCH=amd64 docker-volumes
```

This would make a binary for darwin/amd64 (OSX), available for `docker cp` at the
same location as above.

Alternatively, if you already have golang installed on your system you can
compile it yourself:

```bash
git clone git@github.com:cpuguy83/docker-volumes.git
cd docker-volumes
go get
go build
```

## Usage

Commands:

* **list** - Lists all volumes on the host
* **inspect** - Get details of a volume, takes ID or name from output of `list`
* **rm** - Removes a volume. A volume is only removed if no containers are using it
* **export** - Creates an archive of the volume and outputs it to stdout.  You can
  optionally pause all running containers (which are using the requested volume)
  before exporting the volume using `--pause`

```
NAME:
   docker-volumes - The missing volume manager for Docker

USAGE:
   docker-volumes [global options] command [command options] [arguments...]

VERSION:
   0.1.0

AUTHOR:
  Brian Goff - <cpuguy83@gmail.com>

COMMANDS:
   list		List all volumes
   inspect	Get details of volume
   rm		Delete a volume
   export	Export a as a tarball. Prints to stdout
   help, h	Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --host, -H '/var/run/docker.sock'	Location of the Docker socket [$DOCKER_HOST]
   --help, -h				show help
   --version, -v			print the version
```
