package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/olekukonko/tablewriter"
)

func volumeList(ctx *cli.Context) {
	docker := getDockerClient(ctx)

	volumes := setup(docker)

	var items [][]string
	for _, vol := range volumes.s {
		id := vol.Id()
		if len(id) > 12 {
			id = id[:12]
		}

		out := []string{id, strings.Join(vol.Names, ", "), vol.HostPath}
		items = append(items, out)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Names", "Path"})
	table.SetBorder(false)
	table.AppendBulk(items)
	table.Render()
}

func volumeInspect(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		log.Fatal("Malformed argument. Please supply 1 and only 1 argument")
	}

	docker := getDockerClient(ctx)
	volumes := setup(docker)

	v := volumes.Find(ctx.Args()[0])
	vJson, err := json.MarshalIndent(v, "", "	")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(vJson))
}

func volumeRm(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		log.Fatal("Malformed argument. Please supply 1 and only 1 argument")
	}

	docker := getDockerClient(ctx)
	volumes := setup(docker)

	v := volumes.Find(ctx.Args()[0])
	if !volumes.CanRemove(v) {
		log.Fatal("Volume is in use, cannot remove: ", ctx.Args()[0])
	}

	hostMountPath := strings.TrimSuffix(v.HostPath, filepath.Base(v.HostPath))

	bindSpec := hostMountPath + ":" + "/dockervolumeremove"
	containerConfig := map[string]interface{}{
		"Image": "busybox",
		"Cmd":   "rm -rf /dockervolumeremove/" + filepath.Base(v.HostPath),
		"Volumes": map[string]struct{}{
			"/dockervolumeremove": struct{}{},
		},
		"HostConfig": map[string]interface{}{
			"Binds": []string{bindSpec},
		},
	}

	containerId, err := docker.RunContainer(containerConfig)
	if err != nil {
		docker.RemoveContainer(containerId, true, true)
		log.Fatal("Could not remove volume: ", v.HostPath)
	}
	docker.RemoveContainer(containerId, true, true)

	log.Println("Successfully removed volume: ", ctx.Args()[0])
}

func volumeExport(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		log.Fatal("Malformed argument. Please supply 1 and only 1 argument")
	}
	docker := getDockerClient(ctx)
	volumes := setup(docker)

	name := ctx.Args()[0]
	v := volumes.Find(name)
	if v == nil {
		log.Fatal("Could not find volume: ", name)
	}

	if ctx.Bool("pause") {
		for _, c := range v.Containers {
			err := docker.ContainerPause(c)
			if err != nil {
				docker.ContainerUnpause(c)
				continue
			}
			defer docker.ContainerUnpause(c)
		}
	}

	bindSpec := v.HostPath + ":/.dockervolume"
	containerConfig := map[string]interface{}{
		"Image": "busybox",
		"Cmd":   []string{"/bin/sh", "-c", fmt.Sprintf("cp -r /.dockervolume /%v", v.Id())},
		"Volumes": map[string]struct{}{
			"/.dockervolume": struct{}{},
		},
		"HostConfig": map[string]interface{}{
			"Binds": []string{bindSpec},
		},
	}

	containerId, err := docker.RunContainer(containerConfig)
	if err != nil {
		docker.RemoveContainer(containerId, true, true)
		log.Fatal(containerId, err)
	}

	file, err := docker.Copy(containerId, fmt.Sprintf("/%v", v.Id()))
	if err != nil {
		docker.RemoveContainer(containerId, true, true)
		log.Fatal(err)
	}

	defer docker.RemoveContainer(containerId, true, true)

	io.Copy(os.Stdout, file)
}
