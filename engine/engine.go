package engine

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-etcd/etcd"
	"github.com/samalba/dockerclient"
)

var log = logrus.New()

type (
	Engine struct {
		dockerUrl    string
		etcdMachines []string
		docker       *dockerclient.DockerClient
		etcdClient   *etcd.Client
		rootDomain   string
		hostIP       string
	}
)

func NewEngine(dockerUrl string, etcdMachines []string, rootDomain string, hostIP string) *Engine {
	docker, err := dockerclient.NewDockerClient(dockerUrl)
	if err != nil {
		log.Fatalf("Unable to connect to docker: %s", err)
	}
	etcdClient := etcd.NewClient(etcdMachines)
	e := &Engine{
		dockerUrl:    dockerUrl,
		etcdMachines: etcdMachines,
		docker:       docker,
		etcdClient:   etcdClient,
		rootDomain:   rootDomain,
		hostIP:       hostIP,
	}
	return e
}

func (e *Engine) eventHandler(event *dockerclient.Event, args ...interface{}) {
	cnt, err := e.docker.InspectContainer(event.Id)
	if err != nil {
		log.Warn(err)
		return
	}
	host := fmt.Sprintf("%s.%s", cnt.Config.Hostname, e.rootDomain)
	hostKey := fmt.Sprintf("/vulcand/hosts/" + host)
	up := fmt.Sprintf("up-%s", host)
	upKey := fmt.Sprintf("/vulcand/upstreams/%s", up)
	switch event.Status {
	case "start":
		// for now only get the first port for use with etcd since it would
		// be crazy to have multiple endpoints with varying ports
		for _, v := range cnt.NetworkSettings.Ports {
			m := v[0]
			port := m.HostPort
			cntConn := fmt.Sprintf("http://%s:%s", e.hostIP, port)
			log.WithFields(logrus.Fields{
				"host":     host,
				"endpoint": cntConn,
			}).Info("Adding host")
			// create key structure in etcd
			_, err := e.etcdClient.CreateDir(hostKey, 0)
			if err != nil {
				log.WithFields(logrus.Fields{
					"host":  host,
					"key":   hostKey,
					"error": err,
				}).Error("Error creating key in etcd")
				return
			}
			// set upstream
			upKey := fmt.Sprintf("%s/endpoints/e1", upKey)
			_, err = e.etcdClient.Set(upKey, cntConn, 0)
			if err != nil {
				log.WithFields(logrus.Fields{
					"host":  host,
					"key":   upKey,
					"error": err,
				}).Error("Error creating upstream in etcd")
				return
			}
			// set location
			locKey := fmt.Sprintf("%s/locations/home/path", hostKey)
			_, err = e.etcdClient.Set(locKey, "/.*", 0)
			if err != nil {
				log.WithFields(logrus.Fields{
					"host":  host,
					"key":   locKey,
					"error": err,
				}).Error("Error creating location in etcd")
				return
			}
			locUpKey := fmt.Sprintf("%s/locations/home/upstream", hostKey)
			_, err = e.etcdClient.Set(locUpKey, up, 0)
			if err != nil {
				log.WithFields(logrus.Fields{
					"host":  host,
					"key":   locKey,
					"error": err,
				}).Error("Error creating location upstream in etcd")
				return
			}
			break
		}
	case "die":
		log.WithFields(logrus.Fields{
			"host": host,
		}).Info("Removing host")
		_, err := e.etcdClient.RawDelete(hostKey, true, true)
		if err != nil {
			log.WithFields(logrus.Fields{
				"host":  host,
				"key":   hostKey,
				"error": err,
			}).Error("Error removing host from etcd")
			return
		}
		_, err = e.etcdClient.RawDelete(upKey, true, true)
		if err != nil {
			log.WithFields(logrus.Fields{
				"host":  host,
				"key":   upKey,
				"error": err,
			}).Error("Error removing upstream from etcd")
			return
		}

	}
}

func (e *Engine) Run() {
	log.WithFields(logrus.Fields{
		"ip":     e.hostIP,
		"domain": e.rootDomain,
		"docker": e.dockerUrl,
		"etcd":   e.etcdMachines,
	}).Info("Starting Engine")
	// listen for docker events
	e.docker.StartMonitorEvents(e.eventHandler)
}

func (e *Engine) Stop() {
	log.Info("Stopping Engine")
	e.docker.StopAllMonitorEvents()
}
