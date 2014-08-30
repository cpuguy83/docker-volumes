package main

import (
	"log"
	"os"

	"strings"

	"github.com/codegangsta/cli"
	"github.com/cpuguy83/dockerclient"
)

func main() {
	app := cli.NewApp()
	app.Name = "Docker Volume Admin"
	app.Usage = "The missing volume administrator for Docker"
	//app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "sock, s",
			Value:  "/var/run/docker.sock",
			Usage:  "Location of the Docker socket",
			EnvVar: "DOCKER_HOST",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:   "list",
			Usage:  "List all volumes",
			Action: listVolumes,
		},
		{
			Name:   "inspect",
			Usage:  "Get details of volume",
			Action: inspectVolume,
		},
		{
			Name:   "rm",
			Usage:  "Delete a volume",
			Action: volumeRm,
		},
		{
			Name:   "export",
			Usage:  "Export a as a tarball. Prints to stdout",
			Action: volumeExport,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "pause, p",
					Usage: "Pause any container using the volume before export",
				},
			},
		},
	}

	app.Run(os.Args)
}

func getDockerClient(ctx *cli.Context) docker.Docker {
	docker, err := docker.NewClient(ctx.GlobalString("sock"))
	if err != nil {
		log.Fatal(err)
	}
	return docker
}

func setup(client docker.Docker) *volStore {
	var volumes = &volStore{
		s:      make(map[string]*Volume),
		refMap: make(map[string]map[*docker.Container]struct{}),
	}
	containers, err := client.FetchAllContainers()
	if err != nil {
		log.Fatal(err)
	}

	for _, c := range containers {
		c, err = client.FetchContainer(c.Id)
		if err != nil {
			log.Println(err)
		}
		vols, err := c.GetVolumes()
		if err != nil {
			log.Println("Error pulling volumes for:", c.Id)
		}

		for path, vol := range vols {
			v := &Volume{Volume: vol}

			name := strings.TrimPrefix(c.Name, "/")
			name = name + ":" + path

			if vol, exists := volumes.s[v.Id()]; exists {
				v = vol
			}

			v.Names = append(v.Names, name)
			v.Containers = append(v.Containers, c.Id)

			volumes.Add(v)
			volumes.AddRef(v, c)

		}
	}
	return volumes
}
