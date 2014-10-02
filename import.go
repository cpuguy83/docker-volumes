package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/cpuguy83/dockerclient"
	"github.com/docker/docker/pkg/archive"
)

func buildImportImage(docker docker.Docker, context io.Reader, name string) (string, error) {
	resp, err := docker.Build(context, name, false, true)
	if err != nil {
		return "", err
	}
	msgs := docker.DecodeStream(resp)
	msgs = strings.Split(msgs[len(msgs)-1], " ")
	imgId := msgs[len(msgs)-1]

	return imgId, nil
}

func extractVolConfigJson(imgId string, docker docker.Docker) (string, error) {
	extractVolInfoConfig := map[string]interface{}{
		"Image": imgId,
		"Cmd":   []string{"/bin/sh", "-c", "true"},
	}
	cid1, err := docker.RunContainer(extractVolInfoConfig)
	if err != nil {
		return "", fmt.Errorf("Could not extract volume config: ", err)
	}
	defer docker.RemoveContainer(cid1, true, true)

	tmpArch, err := docker.Copy(cid1, "/.volData/config.json")
	if err != nil {
		return "", fmt.Errorf("Could not extract volume config: ", err)
	}

	// Setup tmp dir for extracting the downloaded archive
	id := GenerateRandomID()
	tmpDir, err := ioutil.TempDir("", id)
	if err != nil {
		return "", fmt.Errorf("Could not create temp dir: ", err)
	}
	defer os.RemoveAll(tmpDir)

	// extract the tar to a temp dir so we can get the inner-tar
	if err := archive.Untar(tmpArch, tmpDir, &archive.TarOptions{Compression: archive.Uncompressed, NoLchown: true}); err != nil {
		return "", fmt.Errorf("Could not untar archive", err)
	}
	// Get the inner-tar and output to stdout
	configFile, err := os.Open(tmpDir + "/config.json")
	if err != nil {
		return "", fmt.Errorf("Could not open config.json: ", err)
	}
	var volConfig Volume
	if err := json.NewDecoder(configFile).Decode(&volConfig); err != nil {
		fmt.Errorf("Could not read config.json: ", err)
	}

	return volConfig.VolPath, nil
}
