package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/olekukonko/tablewriter"
)

func volumeList(ctx *cli.Context) {
	docker := getDockerClient(ctx)

	volumes := setup(docker, ctx.GlobalString("docker-root"))

	if ctx.Bool("quiet") {
		var out []string
		for _, vol := range volumes.s {
			id := vol.Id()
			out = append(out, id)
		}
		fmt.Fprintln(os.Stdout, strings.Join(out, "\n"))
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
		fmt.Fprintln(os.Stderr, "Malformed argument. Please supply 1 and only 1 argument")
		os.Exit(1)
	}

	docker := getDockerClient(ctx)
	volumes := setup(docker, ctx.GlobalString("docker-root"))

	v := volumes.Find(ctx.Args()[0])
	vJson, err := json.MarshalIndent(v, "", "	")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error marshalling volume data: %v", err)
		os.Exit(1)
	}

	fmt.Println(string(vJson))
}

func volumeRm(ctx *cli.Context) {
	if len(ctx.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "Malformed argument. Must supply at least 1 argument")
		os.Exit(1)
	}

	docker := getDockerClient(ctx)
	volumes := setup(docker, ctx.GlobalString("docker-root"))
	for _, name := range ctx.Args() {

		v := volumes.Find(name)
		if v == nil {
			fmt.Fprintln(os.Stderr, "Could not find volume: ", name)
			continue
		}
		if !volumes.CanRemove(v) {
			fmt.Fprintln(os.Stderr, "Volume is in use, cannot remove: ", name)
			continue
		}

		var containerConfig map[string]interface{}

		if dockerApiVersion.LessThan("1.19") {

			hostMountPath := strings.TrimSuffix(v.HostPath, path.Base(v.HostPath))
			hostConfPath := strings.TrimSuffix(hostMountPath, "/vfs/dir/") + "/volumes"

			bindSpec := hostMountPath + ":" + "/.dockervolume"
			bindSpec2 := hostConfPath + ":" + "/.dockervolume2"
			containerConfig = map[string]interface{}{
				"Image":      "busybox:latest",
				"Entrypoint": []string{"/bin/sh", "-c"},
				"Cmd":        []string{"rm -rf /.dockervolume/" + path.Base(v.HostPath) + ("&& rm -rf /.dockervolume2/" + path.Base(v.HostPath))},
				"HostConfig": map[string]interface{}{
					"Binds": []string{bindSpec, bindSpec2},
				},
			}
		} else {
			hostMountPath := strings.TrimSuffix(v.HostPath, path.Base(v.HostPath))
			hostMountPath = strings.TrimSuffix(v.HostPath, path.Base(v.HostPath))
			bindSpec := hostMountPath + ":" + "/.dockervolume"
			containerConfig = map[string]interface{}{
				"Image":      "busybox:latest",
				"Entrypoint": []string{"/bin/sh", "-c"},
				"Cmd":        []string{"rm -rf /.dockervolume/" + path.Base(v.HostPath)},
				"HostConfig": map[string]interface{}{
					"Binds": []string{bindSpec},
				},
			}
		}

		containerId, err := docker.RunContainer(containerConfig)
		if err != nil {
			docker.RemoveContainer(containerId, true, true)
			fmt.Fprintln(os.Stderr, "Could not remove volume: ", v.HostPath)
			continue
		}
		defer docker.RemoveContainer(containerId, true, true)
		docker.ContainerWait(containerId)
		c, err := docker.FetchContainer(containerId)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error getting removeal state")
			os.Exit(1)
		}

		if c.State.ExitCode != 0 {
			fmt.Fprintln(os.Stderr, "Could not remove volume ", v.HostPath)
			fmt.Println(os.Stderr, c.State.Error)
		}

		fmt.Println("Successfully removed volume: ", name)
	}
}

func volumeExport(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		fmt.Fprintln(os.Stderr, "Malformed argument. Please supply 1 and only 1 argument")
		os.Exit(1)
	}
	docker := getDockerClient(ctx)
	volumes := setup(docker, ctx.GlobalString("docker-root"))

	name := ctx.Args()[0]
	v := volumes.Find(name)
	if v == nil {
		fmt.Fprintln(os.Stderr, "Could not find volume: ", name)
		os.Exit(1)
	}

	pause := ctx.Bool("pause")
	unpause := func() {
		if pause {
			for _, c := range v.Containers {
				docker.ContainerUnpause(c)
			}
		}
	}
	if pause {
		pauseContainers(docker, v.Containers)
		defer unpause()
	}

	arch, err := copyForExport(docker, v)
	if err != nil {
		unpause()
		fmt.Fprintln(os.Stderr, "Could not create export archive: ", err)
		os.Exit(1)
	}
	io.Copy(os.Stdout, arch)
}

func volumeImport(ctx *cli.Context) {
	if len(ctx.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "Missing container")
	}
	docker := getDockerClient(ctx)
	buildContext := bufio.NewReader(os.Stdin)

	importToName := ctx.Args()[0]
	container, err := docker.FetchContainer(importToName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not find container to import to:", importToName)
		os.Exit(1)
	}

	imgId, err := buildImportImage(docker, buildContext, importToName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not create import: %s", err)
	}
	defer docker.RemoveImage(imgId, true, false)

	var copyToVolDir string
	if len(ctx.Args()) > 1 {
		// The user asked for the volume to be put in a sepcific dir
		// Let's pull that container and see if there is a volume at that location
		vols, _ := container.GetVolumes()
		for path, vol := range vols {
			if path == ctx.Args()[1] {
				copyToVolDir = vol.HostPath
				break
			}
		}
		// exit here since we don't know what to do since we couldn't find a volume
		// matching the one passed in
		if copyToVolDir == "" {
			docker.RemoveImage(imgId, true, false)
			fmt.Fprintln(os.Stderr, "Did not find a volume matching the path: ", ctx.Args()[1])
			os.Exit(1)
		}
	}

	if copyToVolDir == "" {
		// Need to get the volume config so we know what volume to restore to
		// We could untar the archive from the build context manually, but if it is a
		// large volume, this would not be ideal, especially since now it is already
		// baked into an image
		volPath, err := extractVolConfigJson(imgId, docker)
		if err != nil {
			docker.RemoveImage(imgId, true, false)
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		vols, err := container.GetVolumes()
		if err != nil {
			docker.RemoveImage(imgId, true, false)
			fmt.Fprintln(os.Stderr, "Could not get volume listing for container ", importToName, ": ", err)
		}

		for _, v := range vols {
			if volPath == v.VolPath {
				copyToVolDir = v.HostPath
				break
			}
		}
		if copyToVolDir == "" {
			docker.RemoveImage(imgId, true, false)
			fmt.Fprintln(os.Stderr, "Did not find a volume matching the path: ", volPath)
			os.Exit(1)
		}
	}

	bindSpec := fmt.Sprintf("%s:/.dockervolume", copyToVolDir)
	containerConfig := map[string]interface{}{
		"Image": imgId,
		"HostConfig": map[string]interface{}{
			"Binds": []string{bindSpec},
		},
	}
	id, err := docker.RunContainer(containerConfig)
	if err != nil {
		docker.RemoveImage(imgId, true, false)
		docker.RemoveContainer(id, true, true)
		fmt.Fprintln(os.Stderr, "Could not import data: ", err)
		os.Exit(1)
	}
	docker.ContainerWait(id)
	docker.RemoveImage(imgId, true, false)
	docker.RemoveContainer(id, true, true)
}
