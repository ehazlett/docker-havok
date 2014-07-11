package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/go-etcd/etcd"
	"github.com/ehazlett/dockerclient"
)

var log = logrus.New()

type (
	Engine struct {
		dockerUrl         string
		etcdMachines      []string
		docker            *dockerclient.DockerClient
		etcdClient        *etcd.Client
		rootDomain        string
		rootSubdomain     string
		hostIP            string
		nameRegex         string
		rateLimit         int
		rateLimitBurst    int
		rateLimitVariable string
		connLimit         int
		connLimitVariable string
	}
	RateLimit struct {
		Id         string             `json:"Id"`
		Priority   int                `json:"Priority"`
		Type       string             `json:"Type"`
		Middleware *RequestMiddleware `json:"Middleware"`
	}
	RequestMiddleware struct {
		PeriodSeconds int    `json:"PeriodSeconds"`
		Burst         int    `json:"Burst"`
		Variable      string `json:"Variable"`
		Requests      int    `json:"Requests"`
	}
	ConnectionLimit struct {
		Id         string                `json:"Id"`
		Priority   int                   `json:"Priority"`
		Type       string                `json:"Type"`
		Middleware *ConnectionMiddleware `json:"Middleware"`
	}
	ConnectionMiddleware struct {
		Variable    string `json:"Variable"`
		Connections int    `json:"Connections"`
	}
)

func NewEngine(dockerUrl string, etcdMachines []string, rootDomain string, rootSubdomain string, hostIP string, nameRegex string, rateLimit int, rateLimitVariable string, rateLimitBurst int, connLimit int, connLimitVariable string) *Engine {
	docker, err := dockerclient.NewDockerClient(dockerUrl)
	if err != nil {
		log.Fatalf("Unable to connect to docker: %s", err)
	}
	etcdClient := etcd.NewClient(etcdMachines)
	e := &Engine{
		dockerUrl:         dockerUrl,
		etcdMachines:      etcdMachines,
		docker:            docker,
		etcdClient:        etcdClient,
		rootDomain:        rootDomain,
		rootSubdomain:     rootSubdomain,
		hostIP:            hostIP,
		nameRegex:         nameRegex,
		rateLimit:         rateLimit,
		rateLimitBurst:    rateLimitBurst,
		rateLimitVariable: rateLimitVariable,
		connLimit:         connLimit,
		connLimitVariable: connLimitVariable,
	}
	return e
}

func getHostKey(host string) string {
	hostKey := fmt.Sprintf("/vulcand/hosts/" + host)
	return hostKey
}

func getUpstreamKey(host string) string {
	up := fmt.Sprintf("up-%s", host)
	upKey := fmt.Sprintf("/vulcand/upstreams/%s", up)
	return upKey
}

func getEndpointKey(name, host string) string {
	upKey := getUpstreamKey(host)
	ep := fmt.Sprintf("%s/endpoints", upKey)
	epKey := fmt.Sprintf("%s/%s", ep, name)
	return epKey
}

func (e *Engine) addHost(host string) {
	hostKey := getHostKey(host)
	// create key structure in etcd
	_, er := e.etcdClient.Get(hostKey, false, false)
	if er != nil {
		// check for missing key error
		switch er.(*etcd.EtcdError).ErrorCode {
		case 100:
			// key not found ; create
			_, err := e.etcdClient.CreateDir(hostKey, 0)
			if err != nil {
				log.WithFields(logrus.Fields{
					"host":  host,
					"key":   hostKey,
					"error": err,
				}).Error("Error creating host key in etcd")
				return
			}
		default:
			log.WithFields(logrus.Fields{
				"host":  host,
				"key":   hostKey,
				"error": er,
			}).Error("Error checking host key in etcd")
			return
		}
	}
}

func (e *Engine) addUpstream(host string) {
	hostKey := getHostKey(host)
	locUpKey := fmt.Sprintf("%s/locations/home/upstream", hostKey)
	up := getUpstreamKey(host)
	_, err := e.etcdClient.Set(locUpKey, up, 0)
	if err != nil {
		log.WithFields(logrus.Fields{
			"host":  host,
			"error": err,
		}).Error("Error creating location upstream in etcd")
		return
	}
}

func (e *Engine) addEndpoint(name, host, endpoint string) {
	epKey := getEndpointKey(name, host)
	_, err := e.etcdClient.Set(epKey, endpoint, 0)
	if err != nil {
		log.WithFields(logrus.Fields{
			"host":     host,
			"endpoint": endpoint,
			"error":    err,
		}).Error("Error creating endpoint in etcd")
		return
	}
}

func (e *Engine) removeEndpoint(name, host string) {
	epKey := getEndpointKey(name, host)
	_, err := e.etcdClient.RawDelete(epKey, true, true)
	if err != nil {
		log.WithFields(logrus.Fields{
			"host":  host,
			"error": err,
		}).Error("Error removing endpoint from etcd")
		return
	}
}

func (e *Engine) removeUpstream(host string) {
	upKey := getUpstreamKey(host)
	_, err := e.etcdClient.RawDelete(upKey, true, true)
	if err != nil {
		log.WithFields(logrus.Fields{
			"host":  host,
			"key":   upKey,
			"error": err,
		}).Error("Error removing upstream from etcd")
	}
}

func (e *Engine) removeHost(host string) {
	hostKey := getHostKey(host)
	_, err := e.etcdClient.RawDelete(hostKey, true, true)
	if err != nil {
		log.WithFields(logrus.Fields{
			"host":  host,
			"key":   hostKey,
			"error": err,
		}).Error("Error removing host from etcd")
	}
}

func (e *Engine) addLocation(host string) {
	hostKey := getHostKey(host)
	locKey := fmt.Sprintf("%s/locations/home/path", hostKey)
	_, err := e.etcdClient.Set(locKey, "/.*", 0)
	if err != nil {
		log.WithFields(logrus.Fields{
			"host":  host,
			"key":   locKey,
			"error": err,
		}).Error("Error creating location in etcd")
		return
	}
}

func (e *Engine) setRateLimit(host string) {
	hostKey := getHostKey(host)
	locRateLimitKey := fmt.Sprintf("%s/locations/home/middlewares/ratelimit/default", hostKey)
	cm := &RequestMiddleware{
		PeriodSeconds: 1,
		Burst:         e.rateLimitBurst,
		Variable:      e.rateLimitVariable,
		Requests:      e.rateLimit,
	}
	cl := &RateLimit{
		Id:         "",
		Priority:   1,
		Type:       "ratelimit",
		Middleware: cm,
	}
	b, err := json.Marshal(cl)
	if err != nil {
		log.WithFields(logrus.Fields{
			"host":  host,
			"error": err,
		}).Error("Error setting rate limit config in etcd")
		return
	}
	_, err = e.etcdClient.Set(locRateLimitKey, string(b), 0)
	if err != nil {
		log.WithFields(logrus.Fields{
			"host":  host,
			"error": err,
		}).Error("Error creating location in etcd")
		return
	}
}

func (e *Engine) setConnectionLimit(host string) {
	hostKey := getHostKey(host)
	locConnLimitKey := fmt.Sprintf("%s/locations/home/middlewares/connlimit/default", hostKey)
	rm := &ConnectionMiddleware{
		Variable:    e.connLimitVariable,
		Connections: e.connLimit,
	}
	rl := &ConnectionLimit{
		Id:         "",
		Priority:   1,
		Type:       "connlimit",
		Middleware: rm,
	}
	b, err := json.Marshal(rl)
	if err != nil {
		log.WithFields(logrus.Fields{
			"host":  host,
			"error": err,
		}).Error("Error setting connection limit config in etcd")
		return
	}
	_, err = e.etcdClient.Set(locConnLimitKey, string(b), 0)
	if err != nil {
		log.WithFields(logrus.Fields{
			"host":  host,
			"error": err,
		}).Error("Error creating location in etcd")
		return
	}
}

func (e *Engine) eventHandler(event *dockerclient.Event, args ...interface{}) {
	cnt, err := e.docker.InspectContainer(event.Id)
	if err != nil {
		log.Warn(err)
		return
	}
	name := cnt.Name[1:]
	matched, err := regexp.MatchString(e.nameRegex, name)
	if err != nil {
		log.Errorf("Error matching container: %s", err)
		return
	}
	// if not a match, return immediately
	if !matched {
		return
	}
	host := fmt.Sprintf("%s.%s", cnt.Config.Hostname, e.rootDomain)
	// check for root level domain container
	containerHostParts := []string{cnt.Config.Hostname, cnt.Config.Domainname}
	containerHost := strings.Join(containerHostParts, ".")
	hosts := []string{host}
	if containerHost == e.rootDomain {
		subdomainHost := fmt.Sprintf("%s.%s", e.rootSubdomain, containerHost)
		h := []string{containerHost, subdomainHost}
		hosts = h
	}
	for _, host := range hosts {
		switch event.Status {
		case "start":
			// for now only get the first port for use with etcd since it would
			// be crazy to have multiple endpoints with varying ports
			for _, v := range cnt.NetworkSettings.Ports {
				// check for exposed ports ; if none, report error
				if len(v) == 0 {
					log.WithFields(logrus.Fields{
						"host":      host,
						"container": cnt.Id,
					}).Error("Unable to add endpoint; no ports exposed")
					return
				}
				m := v[0]
				port := m.HostPort
				cntConn := fmt.Sprintf("http://%s:%s", e.hostIP, port)
				log.WithFields(logrus.Fields{
					"host":     host,
					"endpoint": cntConn,
				}).Info("Adding endpoint for host")
				e.addHost(host)
				// set endpoint
				e.addEndpoint(name, host, cntConn)
				// set location
				e.addLocation(host)
				// rate limit
				if e.rateLimit > 0 {
					e.setRateLimit(host)
				}
				// conn limit
				if e.connLimit > 0 {
					e.setConnectionLimit(host)
				}
				// upstream
				e.addUpstream(host)
				break
			}
		case "die", "destroy", "stop":
			// since die is called upon stop as well, only log if "die"
			if event.Status == "die" {
				log.WithFields(logrus.Fields{
					"host": host,
				}).Info("Removing endpoint for host")
			}
			e.removeEndpoint(name, host)
			// check for any other endpoints and break if they exist
			upKey := getUpstreamKey(host)
			ep := fmt.Sprintf("%s/endpoints", upKey)
			r, er := e.etcdClient.Get(ep, true, true)
			if er != nil {
				log.WithFields(logrus.Fields{
					"host":  host,
					"error": er,
				}).Error("Error checking endpoint from etcd")
				return
			}
			// if there are no more nodes, cleanup
			if len(r.Node.Nodes) == 0 {
				// if no more endpoints (all are gone) then remove upstream and host
				e.removeUpstream(host)
				log.WithFields(logrus.Fields{
					"host": host,
				}).Info("Removing host")
				// remove host
				e.removeHost(host)
			}
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
