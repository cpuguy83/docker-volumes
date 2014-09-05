package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/cpuguy83/dockerclient"
	"github.com/docker/docker/archive"
)

func pauseContainers(docker docker.Docker, containers []string) {
	for _, c := range containers {
		err := docker.ContainerPause(c)
		if err != nil {
			docker.ContainerUnpause(c)
			continue
		}
	}
}

var ExportDockerfile = `
FROM busybox
ADD data /.volData
ADD config.json /.volData/config.json
CMD rm /.volData/config.json && cp -r /.volData/* /.dockervolume/
`

func copyForExport(docker docker.Docker, v *Volume) (io.Reader, error) {
	bindSpec := v.HostPath + ":/.dockervolume"

	vJson, err := json.MarshalIndent(v, "", "	")
	if err != nil {
		return nil, fmt.Errorf("Could not export volume data")
	}
	jsonStr := string(vJson)

	// Since we're using busybox's tar, it does not support appending files
	// Instead we'll handle adding in Dockerfile/config.json manually
	cmd := fmt.Sprintf(
		"mkdir -p /volumeData && cp -r /.dockervolume /volumeData/data && echo '%s' > /volumeData/Dockerfile && echo '%s' > /volumeData/config.json; cd /volumeData && tar -cf volume.tar .",
		ExportDockerfile,
		jsonStr,
	)
	containerConfig := map[string]interface{}{
		"Image": "busybox",
		"Cmd":   []string{"/bin/sh", "-c", cmd},
		"Volumes": map[string]struct{}{
			"/.dockervolume": struct{}{},
		},
		"HostConfig": map[string]interface{}{
			"Binds": []string{bindSpec},
		},
	}

	containerId, err := docker.RunContainer(containerConfig)
	if err != nil {
		return nil, fmt.Errorf("%s - %s", containerId, err)
	}
	defer docker.RemoveContainer(containerId, true, true)

	// Wait for the container to exit, signaling that our archive is ready
	if err := docker.ContainerWait(containerId); err != nil {
		return nil, fmt.Errorf("Could not get archive: %s", err)
	}

	// This is a tar of a tar, we only want the inner tar, so do some more stuff
	tmpArch, err := docker.Copy(containerId, "/volumeData/volume.tar")
	if err != nil {
		return nil, fmt.Errorf("Could not get archive: %s", err)
	}

	id := GenerateRandomID()
	tmpDir, err := ioutil.TempDir("", id)
	if err != nil {
		return nil, fmt.Errorf("Could not create temp dir: %s", err)
	}
	defer os.RemoveAll(tmpDir)
	// extract the tar to a temp dir so we can get the inner-tar
	if err := archive.Untar(tmpArch, tmpDir, &archive.TarOptions{Compression: archive.Uncompressed, NoLchown: true}); err != nil {
		return nil, fmt.Errorf("Could not untar archive: %s", err)
	}
	// Get the inner-tar and output to stdout
	return os.Open(tmpDir + "/volume.tar")
}
