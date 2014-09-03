package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"strings"

	"github.com/codegangsta/cli"
	"github.com/cpuguy83/dockerclient"
)

func main() {
	app := cli.NewApp()
	app.Name = "Docker Volume Manager"
	app.Usage = "The missing volume manager for Docker"
	app.Action = volumeList
	app.Version = "0.1.0"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Value:  "/var/run/docker.sock",
			Usage:  "Location of the Docker socket",
			EnvVar: "DOCKER_HOST",
		},
		cli.StringFlag{
			Name:  "mode, m",
			Value: "container",
			Usage: "Set the mode to use, contaienr or host.",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:   "list",
			Usage:  "List all volumes",
			Action: volumeList,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "quiet, q",
					Usage: "Display only IDs",
				},
			},
		},
		{
			Name:   "inspect",
			Usage:  "Get details of volume",
			Action: volumeInspect,
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
	docker, err := docker.NewClient(ctx.GlobalString("host"))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return docker
}

func setup(client docker.Docker) *volStore {
	var volumes = &volStore{
		s: make(map[string]*Volume),
	}
	containers, err := client.FetchAllContainers()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, c := range containers {
		c, err = client.FetchContainer(c.Id)
		if err != nil {
			fmt.Println(err)
		}
		vols, err := c.GetVolumes()
		if err != nil {
			fmt.Println("Error pulling volumes for:", c.Id)
		}

		for path, vol := range vols {
			v := &Volume{Volume: *vol}

			name := strings.TrimPrefix(c.Name, "/")
			name = name + ":" + path

			if vol, exists := volumes.s[v.Id()]; exists {
				v = vol
			}

			v.Names = append(v.Names, name)
			v.Containers = append(v.Containers, c.Id)

			volumes.Add(v)
		}
	}

	info, err := client.Info()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	path := info.RootPath()
	path = strings.TrimSuffix(path, "/"+filepath.Base(path))
	path = path + "/vfs/dir"

	volDirs, err := volumesFromDisk(path, client)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, d := range volDirs {
		hostPath := path + "/" + d

		if hostPath == path+"/" {
			continue
		}

		v := &Volume{Volume: docker.Volume{HostPath: hostPath, IsBindMount: false, IsReadWrite: true}}
		vol := volumes.Find(d)
		if vol != nil {
			continue
		}

		volumes.Add(v)
	}

	return volumes
}

func volumesFromDisk(path string, client docker.Docker) ([]string, error) {
	bindSpec := path + ":" + "/.docker_root"
	containerConfig := map[string]interface{}{
		"Image": "busybox",
		"Cmd":   []string{"/bin/sh", "-c", "ls /.docker_root/"},
		"Volumes": map[string]struct{}{
			"/.docker_root": struct{}{},
		},
		"HostConfig": map[string]interface{}{
			"Binds": []string{bindSpec},
		},
	}

	id, err := client.RunContainer(containerConfig)
	defer client.RemoveContainer(id, true, true)
	if err != nil {
		return nil, err
	}

	dirs, err := client.ContainerLogs(id, false, true, true, false, -1)
	if err != nil {
		return nil, err
	}

	var out []string
	var b []byte

	scanner := bufio.NewScanner(dirs)
	for scanner.Scan() {
		b = append(b, scanner.Bytes()...)
	}

	scanner = bufio.NewScanner(bufio.NewReader(bytes.NewReader(b)))
	scanner.Split(scanHeader)
	for scanner.Scan() {
		out = append(out, strings.Split(scanner.Text(), "\u0001")[0])
	}
	return out, nil
}

func scanHeader(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.IndexByte(data, 'A'); i >= 0 {
		return i + 1, data[0:i], nil
	}

	if atEOF {
		return len(data), data, nil
	}

	return 0, nil, nil
}
