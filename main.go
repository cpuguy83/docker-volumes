package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"strings"

	"github.com/codegangsta/cli"
	"github.com/cpuguy83/dockerclient"
	"github.com/docker/docker/pkg/version"
)

var dockerApiVersion version.Version

func main() {
	app := cli.NewApp()
	app.Name = "docker-volumes"
	app.Usage = "The missing volume manager for Docker"
	app.Version = "1.2"
	app.Author = "Brian Goff"
	app.Email = "cpuguy83@gmail.com"
	certPath := os.Getenv("DOCKER_CERT_PATH")
	if certPath == "" {
		certPath = filepath.Join(os.Getenv("HOME"), ".docker")
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Value:  "/var/run/docker.sock",
			Usage:  "Location of the Docker socket",
			EnvVar: "DOCKER_HOST",
		},
		cli.BoolFlag{
			Name:   "tls",
			Usage:  "Enable TLS",
			EnvVar: "DOCKER_TLS",
		},
		cli.StringFlag{
			Name:   "tlsverify",
			Usage:  "Enable TLS Server Verification",
			EnvVar: "DOCKER_TLS_VERIFY",
		},
		cli.StringFlag{
			Name:  "tlscacert",
			Value: filepath.Join(certPath, "ca.pem"),
			Usage: "Location of tls ca cert",
		},
		cli.StringFlag{
			Name:  "tlscert",
			Value: filepath.Join(certPath, "cert.pem"),
			Usage: "Location of tls cert",
		},
		cli.StringFlag{
			Name:  "tlskey",
			Value: filepath.Join(certPath, "key.pem"),
			Usage: "Location of tls key",
		},
		cli.StringFlag{
			Name:  "docker-root",
			Value: "/var/lib/docker",
			Usage: "Location of the Docker root path",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:    "list",
			Aliases: []string{"ls"},
			Usage:   "List all volumes",
			Action:  volumeList,
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
		{
			Name:   "import",
			Usage:  "Import a tarball produced by the export command the specified container",
			Action: volumeImport,
		},
	}

	app.Run(os.Args)
}

func getDockerClient(ctx *cli.Context) docker.Docker {
	docker, err := docker.NewClient(ctx.GlobalString("host"))
	var tlsConfig tls.Config
	tlsConfig.InsecureSkipVerify = true
	if ctx.GlobalBool("tls") || ctx.GlobalString("tlsverify") != "" {
		if ctx.GlobalString("tlsverify") != "" {
			certPool := x509.NewCertPool()
			file, err := ioutil.ReadFile(ctx.GlobalString("tlscacert"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			certPool.AppendCertsFromPEM(file)
			tlsConfig.RootCAs = certPool
			tlsConfig.InsecureSkipVerify = false
		}

		_, errCert := os.Stat(ctx.GlobalString("tlscert"))
		_, errKey := os.Stat(ctx.GlobalString("tlskey"))
		if errCert == nil || errKey == nil {
			cert, err := tls.LoadX509KeyPair(ctx.GlobalString("tlscert"), ctx.GlobalString("tlskey"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Couldn't load X509 key pair: %s. Key encrpyted?\n", err)
				os.Exit(1)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}
		tlsConfig.MinVersion = tls.VersionTLS10
		docker.SetTlsConfig(&tlsConfig)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return docker
}

func setup(client docker.Docker, rootPath string) *volStore {
	ver, err := client.Version()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting docker daemon version")
		os.Exit(1)
	}
	dockerApiVersion = version.Version(ver.ApiVersion)

	var volumes = &volStore{
		s: make(map[string]*Volume),
	}
	containers, err := client.FetchAllContainers(true)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error fetching containers: %v", err)
		os.Exit(1)
	}

	for _, c := range containers {
		c, err = client.FetchContainer(c.Id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error getting container: %v", err)
		}
		vols, err := c.GetVolumes()
		if err != nil {
			fmt.Println("Error pulling volumes for:", c.Id)
		}

		for p, vol := range vols {
			v := &Volume{Volume: *vol}

			name := strings.TrimPrefix(c.Name, "/")
			name = name + ":" + p

			v.ID = v.Id()
			if v.ID == "_data" {
				v.ID = path.Base(path.Dir(v.HostPath))
			}

			if strings.HasSuffix(v.HostPath, "_data") && dockerApiVersion.GreaterThan("1.18") && !v.IsBindMount {
				v.HostPath = path.Dir(v.HostPath)
			}
			if vol, exists := volumes.s[v.ID]; exists {
				v = vol
			}

			v.Names = append(v.Names, name)
			v.Containers = append(v.Containers, c.Id)

			volumes.Add(v)
		}
	}

	volsPath := path.Join(rootPath, "vfs", "dir")
	if dockerApiVersion.GreaterThanOrEqualTo("1.19") {
		volsPath = path.Join(rootPath, "volumes")
	}

	volDirs, err := volumesFromDisk(volsPath, client)
	if err != nil {
		fmt.Fprint(os.Stderr, "error getting volume list: %v", err)
		os.Exit(1)
	}

	for _, d := range volDirs {
		hostPath := path.Join(volsPath, d)

		if hostPath == volsPath || hostPath == volsPath+"/" {
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
		"Image": "busybox:latest",
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

	if err := client.ContainerWait(id); err != nil {
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
