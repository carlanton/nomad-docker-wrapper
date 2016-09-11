package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"sync"
)

const (
	DOCKER_SOCKET  = "/var/run/docker.sock"
	WRAPPER_SOCKET = "/tmp/nomad-docker-wrapper.sock"
)

var (
	envRegexp             = regexp.MustCompile(`^DOCKER_BIND_MOUNT.*=(.+)$`)
	containerCreateRegexp = regexp.MustCompile(`^(/v[0-9\\.]*)?/containers/create$`)
)

type ProxyHandler struct {
	httpProxy http.Handler
}

func NewProxyHandler(dockerSocket string) *ProxyHandler {
	dummyUrl, _ := url.Parse("http://127.0.0.1:0")
	proxy := httputil.NewSingleHostReverseProxy(dummyUrl)
	proxy.Transport = &http.Transport{
		Dial: func(proto, addr string) (net.Conn, error) {
			return net.Dial("unix", dockerSocket)
		},
	}

	return &ProxyHandler{
		httpProxy: proxy,
	}
}

func massageRequestBody(requestBody []byte) ([]byte, error) {
	var rootNode jsonObject
	d := json.NewDecoder(bytes.NewReader(requestBody))
	d.UseNumber()

	if err := d.Decode(&rootNode); err != nil {
		return nil, fmt.Errorf("Unmarshal of request body failed: %s", err)
	}

	envVars, err := rootNode.StringArray("Env")
	if err != nil {
		return nil, err
	}

	newEnv := make([]string, 0)
	foundVolumes := make([]string, 0)

	for _, envVar := range envVars {
		match := envRegexp.FindStringSubmatch(envVar)
		if match != nil && len(match) == 2 {
			foundVolumes = append(foundVolumes, match[1])
		} else {
			newEnv = append(newEnv, envVar)
		}
	}

	if len(foundVolumes) == 0 {
		return requestBody, nil
	}

	rootNode["Env"] = newEnv

	hostConfig, err := rootNode.Object("HostConfig")
	if err != nil {
		return nil, err
	}

	binds, err := hostConfig.StringArray("Binds")
	if err != nil {
		return nil, err
	}

	for _, volume := range foundVolumes {
		binds = append(binds, volume)
	}
	hostConfig["Binds"] = binds

	return json.Marshal(rootNode)
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if containerCreateRegexp.MatchString(r.URL.Path) {
		requestBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Fatal(err)
		}
		if newBody, err := massageRequestBody(requestBody); err != nil {
			log.Printf("Failed to modify request body: %v", err)
		} else {
			r.Body = ioutil.NopCloser(bytes.NewReader(newBody))
			r.ContentLength = int64(len(newBody))
		}
	}

	if r.Header.Get("Upgrade") == "tcp" {
		h.proxyTCP(w, r)
	} else {
		h.httpProxy.ServeHTTP(w, r)
	}
}

func (h *ProxyHandler) proxyTCP(w http.ResponseWriter, r *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}

	serverConn, err := net.Dial("unix", DOCKER_SOCKET)
	if err != nil {
		log.Fatal(err)
	}

	r.Write(serverConn)

	clientConn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go pipe(clientConn, serverConn, &wg)
	go pipe(serverConn, clientConn, &wg)

	wg.Wait()
}

func pipe(dst, src net.Conn, wg *sync.WaitGroup) {
	io.Copy(dst, src)
	dst.Close()
	src.Close()
	wg.Done()
}

func main() {
	if err := os.Remove(WRAPPER_SOCKET); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Error removing existing unix socket: %s", err)
	}

	l, err := net.Listen("unix", WRAPPER_SOCKET)
	if err != nil {
		log.Fatalf("Listen error:", err)
	}

	server := &http.Server{
		Handler: NewProxyHandler(DOCKER_SOCKET),
	}

	log.Fatal(server.Serve(l))
}
