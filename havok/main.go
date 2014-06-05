package main

import (
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/ehazlett/havok/engine"
)

var (
	ETCD_MACHINES_CONNECTION string
	ETCD_MACHINES            []string
	DOCKER_URL               string
	ROOT_DOMAIN              string
	HOST_IP                  string
	NAME_REGEX               string
	eng                      *engine.Engine
	log                      = logrus.New()
	version                  = "0.1"
)

func waitForInterrupt() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	for _ = range sigChan {
		// stop engine
		eng.Stop()
		os.Exit(0)
	}
}

func init() {
	flag.StringVar(&DOCKER_URL, "docker", "unix:///var/run/docker.sock", "Docker URL")
	flag.StringVar(&ROOT_DOMAIN, "root-domain", "local", "Root level domain")
	flag.StringVar(&ETCD_MACHINES_CONNECTION, "etcd-machines", "http://127.0.0.1:4001", "comma separated list of etcd hosts")
	flag.StringVar(&HOST_IP, "host-ip", "127.0.0.1", "Host IP for accessing containers")
	flag.StringVar(&NAME_REGEX, "names", ".*", "Containers with name matching regex will get added to etcd")
	flag.Parse()
	// parse etcd to list
	ETCD_MACHINES = strings.Split(ETCD_MACHINES_CONNECTION, ",")
}

func main() {
	log.Infof("Havok %s", version)
	eng = engine.NewEngine(DOCKER_URL, ETCD_MACHINES, ROOT_DOMAIN, HOST_IP, NAME_REGEX)
	eng.Run()
	waitForInterrupt()
}
