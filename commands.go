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

	if ctx.Bool("quiet") {
		var out []string
		for _, vol := range volumes.s {
			id := vol.Id()
			out = append(out, id)
		}
		fmt.Println(strings.Join(out, "\n"))
		return
	}
	var items [][]string
	for _, vol := range volumes.s {
		id := vol.ID
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
	if len(ctx.Args()) == 0 {
		log.Fatal("Malformed argument. Please supply 1 and only 1 argument")
	}

	for _, name := range ctx.Args() {
		docker := getDockerClient(ctx)
		volumes := setup(docker)

		v := volumes.Find(name)
		if v == nil {
			log.Println("Could not find volume: ", name)
			continue
		}
		if !volumes.CanRemove(v) {
			log.Println("Volume is in use, cannot remove: ", name)
			continue
		}

		hostMountPath := strings.TrimSuffix(v.HostPath, filepath.Base(v.HostPath))
		hostConfPath := strings.TrimSuffix(hostMountPath, "/vfs/dir/") + "/volumes"

		bindSpec := hostMountPath + ":" + "/.dockervolume"
		bindSpec2 := hostConfPath + ":" + "/.dockervolume2"
		containerConfig := map[string]interface{}{
			"Image": "busybox",
			"Cmd":   []string{"/bin/sh", "-c", ("rm -rf /.dockervolume/" + filepath.Base(v.HostPath) + ("&& rm -rf /.dockervolume2/" + filepath.Base(v.HostPath)))},
			"Volumes": map[string]struct{}{
				"/.dockervolume":  struct{}{},
				"/.dockervolume2": struct{}{},
			},
			"HostConfig": map[string]interface{}{
				"Binds": []string{bindSpec, bindSpec2},
			},
		}

		containerId, err := docker.RunContainer(containerConfig)
		if err != nil {
			docker.RemoveContainer(containerId, true, true)
			log.Println("Could not remove volume: ", v.HostPath)
			continue
		}
		docker.RemoveContainer(containerId, true, true)

		log.Println("Successfully removed volume: ", name)
	}
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
	defer docker.RemoveContainer(containerId, true, true)

	file, err := docker.Copy(containerId, fmt.Sprintf("/%v", v.Id()))
	if err != nil {
		docker.RemoveContainer(containerId, true, true)
		log.Fatal(err)
	}

	io.Copy(os.Stdout, file)
}
