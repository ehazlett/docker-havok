package main

import (
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/ehazlett/docker-havok/engine"
)

var (
	ETCD_MACHINES_CONNECTION string
	ETCD_MACHINES            []string
	DOCKER_URL               string
	ROOT_DOMAIN              string
	HOST_IP                  string
	NAME_REGEX               string
	RATE_LIMIT               int
	RATE_LIMIT_VARIABLE      string
	RATE_LIMIT_BURST         int
	CONN_LIMIT               int
	CONN_LIMIT_VARIABLE      string
	eng                      *engine.Engine
	log                      = logrus.New()
	version                  = "0.4"
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
	flag.IntVar(&RATE_LIMIT, "rate-limit", 0, "Specify rate limit as requests per second (default: 0)")
	flag.StringVar(&RATE_LIMIT_VARIABLE, "rate-limit-var", "client.ip", "Variable for rate limiting (default: client.ip)")
	flag.IntVar(&RATE_LIMIT_BURST, "rate-limit-burst", 1, "Specify rate limit burst (default: 1)")
	flag.IntVar(&CONN_LIMIT, "conn-limit", 0, "Specify connection limit (default: 0)")
	flag.StringVar(&CONN_LIMIT_VARIABLE, "conn-limit-var", "client.ip", "Variable for connection limiting (default: client.ip)")
	flag.Parse()
	// parse etcd to list
	ETCD_MACHINES = strings.Split(ETCD_MACHINES_CONNECTION, ",")
}

func main() {
	log.Infof("Havok %s", version)
	eng = engine.NewEngine(DOCKER_URL, ETCD_MACHINES, ROOT_DOMAIN, HOST_IP, NAME_REGEX, RATE_LIMIT, RATE_LIMIT_VARIABLE, RATE_LIMIT_BURST, CONN_LIMIT, CONN_LIMIT_VARIABLE)
	eng.Run()
	waitForInterrupt()
}
