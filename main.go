package main

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
)

const (
	DEFAULT_DOCKER_SOCKET  = "/var/run/docker.sock"
	DEFAULT_WRAPPER_SOCKET = "/tmp/nomad-docker-wrapper.sock"
)

var (
	version = "DEV"
	githash = "hash"
)

func main() {
	log.SetFlags(0)
	log.Printf("nomad-docker-wrapper version %s-%s", version, githash)

	dockerSocket := DEFAULT_DOCKER_SOCKET
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost != "" {
		u, err := url.Parse(dockerHost)
		if err != nil {
			log.Fatalf("Couldn't parse DOCKER_HOST: %s", err)
		} else if u.Scheme != "unix" {
			log.Fatalf("Unsupported Docker socket scheme: %s", u.Scheme)
		}
		dockerSocket = u.Path
	}

	wrapperSocket := os.Getenv("WRAPPER_SOCKET")
	if wrapperSocket == "" {
		wrapperSocket = DEFAULT_WRAPPER_SOCKET
	}

	log.Printf("Configuration: dockerSocket=%s, wrapperSocket=%s",
		dockerSocket, wrapperSocket)

	if err := os.Remove(wrapperSocket); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Error removing existing unix socket: %s", err)
	}
	l, err := net.Listen("unix", wrapperSocket)
	if err != nil {
		log.Fatalf("Listen error: %s", err)
	}

	dockerSocketStat, err := os.Stat(dockerSocket)
	if err != nil {
		log.Fatalf("Could not stat socket: %s")
	}
	dockerSocketStat_t, ok := dockerSocketStat.Sys().(*syscall.Stat_t)
	if !ok {
		log.Fatalf("Type assertion failed! Unsupported OS?")
	}
	// Use the same mode, uid and gid for the wrapper socket
	if err := os.Chmod(wrapperSocket, dockerSocketStat.Mode()); err != nil {
		log.Fatalf("Could't chmod wrapper socket: %s", err)
	}
	uid := int(dockerSocketStat_t.Uid)
	gid := int(dockerSocketStat_t.Gid)
	if err := os.Chown(wrapperSocket, uid, gid); err != nil {
		log.Fatalf("Couldn't chown wrapper socket: %s", err)
	}

	server := &http.Server{
		Handler: NewProxyHandler(dockerSocket),
	}

	log.Fatal(server.Serve(l))
}
