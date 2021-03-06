package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/libnetwork/ipamapi"
	weaveapi "github.com/weaveworks/weave/api"
	. "github.com/weaveworks/weave/common"
	"github.com/weaveworks/weave/common/docker"
	weavenet "github.com/weaveworks/weave/net"
	ipamplugin "github.com/weaveworks/weave/plugin/ipam"
	netplugin "github.com/weaveworks/weave/plugin/net"
	"github.com/weaveworks/weave/plugin/skel"
)

var version = "(unreleased version)"

func main() {
	var (
		justVersion      bool
		address          string
		meshAddress      string
		logLevel         string
		noMulticastRoute bool
	)

	flag.BoolVar(&justVersion, "version", false, "print version and exit")
	flag.StringVar(&logLevel, "log-level", "info", "logging level (debug, info, warning, error)")
	flag.StringVar(&address, "socket", "/run/docker/plugins/weave.sock", "socket on which to listen")
	flag.StringVar(&meshAddress, "meshsocket", "/run/docker/plugins/weavemesh.sock", "socket on which to listen in mesh mode")
	flag.BoolVar(&noMulticastRoute, "no-multicast-route", false, "do not add a multicast route to network endpoints")

	flag.Parse()

	if justVersion {
		fmt.Printf("weave plugin %s\n", version)
		os.Exit(0)
	}

	SetLogLevel(logLevel)

	// API 1.21 is the first version that supports docker network commands
	dockerClient, err := docker.NewVersionedClientFromEnv("1.21")
	if err != nil {
		Log.Fatalf("unable to connect to docker: %s", err)
	}

	weave := weaveapi.NewClient(os.Getenv("WEAVE_HTTP_ADDR"), Log)

	Log.Println("Weave plugin", version, "Command line options:", os.Args[1:])
	Log.Info(dockerClient.Info())

	err = run(dockerClient, weave, address, meshAddress, noMulticastRoute)
	if err != nil {
		Log.Fatal(err)
	}
}

func run(dockerClient *docker.Client, weave *weaveapi.Client, address, meshAddress string, noMulticastRoute bool) error {
	endChan := make(chan error, 1)
	if address != "" {
		globalListener, err := listenAndServe(dockerClient, weave, address, noMulticastRoute, endChan, "global", false)
		if err != nil {
			return err
		}
		defer os.Remove(address)
		defer globalListener.Close()
	}
	if meshAddress != "" {
		meshListener, err := listenAndServe(dockerClient, weave, meshAddress, noMulticastRoute, endChan, "local", true)
		if err != nil {
			return err
		}
		defer os.Remove(meshAddress)
		defer meshListener.Close()
	}

	statusListener, err := weavenet.ListenUnixSocket("/home/weave/status.sock")
	if err != nil {
		return err
	}
	go serveStatus(statusListener)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		Log.Debugf("Caught signal %s; shutting down", sig)
		return nil
	case err := <-endChan:
		return err
	}
}

func listenAndServe(dockerClient *docker.Client, weave *weaveapi.Client, address string, noMulticastRoute bool, endChan chan<- error, scope string, withIpam bool) (net.Listener, error) {
	d, err := netplugin.New(dockerClient, weave, scope, noMulticastRoute)
	if err != nil {
		return nil, err
	}

	var i ipamapi.Ipam
	if withIpam {
		i = ipamplugin.NewIpam(weave)
	}

	listener, err := weavenet.ListenUnixSocket(address)
	if err != nil {
		return nil, err
	}
	Log.Printf("Listening on %s for %s scope", address, scope)

	go func() {
		endChan <- skel.Listen(listener, d, i)
	}()

	return listener, nil
}

func serveStatus(listener net.Listener) {
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})}
	if err := server.Serve(listener); err != nil {
		Log.Fatalf("ListenAndServeStatus failed: %s", err)
	}
}
