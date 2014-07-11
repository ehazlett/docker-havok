# Havok
Havok is a bridge between Docker and [Vulcand](https://github.com/mailgun/vulcand).  It works by listening for Docker events and automatically creating hosts and upstreams in etcd which are then used by Vulcand to serve the app.

Using the `-names` option, you can restrict which containers have upstreams created for them thus only exposing the containers you want to Vulcand.  Also, the endpoints are generated in etcd based upon the container name.  This allows you to run Havok on multiple hosts  all pointing to the same etcd cluster and have containers distributed amongst hosts.  When there are no more endpoints available, Havok will remove the host from Vulcand.

# Demo

[![Havok](http://img.youtube.com/vi/jimFfpKZvT0/0.jpg)](http://www.youtube.com/watch?v=jimFfpKZvT0)

## Assumptions
Currently there are some assumtions:

* Havok will only use the first exposed port (multiple ports to differing services would cause mayhem)
* The hostname of the container will be used as the subdomain (see the `-root-domain` setting below)

# Usage
You must have etcd, vulcand, and docker (obviously) to use Havok.  Here are some quick instructions:

Start etcd:

`docker run -d -p 4001:4001 -p 7001:7001 coreos/etcd`

Start vulcand (replace the `1.2.3.4` IP with your non-local machine IP (i.e. 192.168.x.x)):

`docker run -d -p 80:80 -p 8182:8182 mailgun/vulcand /opt/vulcan/vulcand -apiInterface="0.0.0.0" -interface="0.0.0.0" -port 80 --etcd=http://1.2.3.4:4001`

Start havok (replace the `1.2.3.4` IP with your non-local machine IP (i.e. 192.168.x.x)):

`docker run --rm -v /var/run/docker.sock:/var/run/docker.sock ehazlett/havok -etcd-machines "http://1.2.3.4:4001" -host-ip 1.2.3.4 -root-domain local`

Start havok with rate and connection limiting

`docker run --rm -v /var/run/docker.sock:/var/run/docker.sock ehazlett/havok -etcd-machines "http://1.2.3.4:4001" -host-ip 1.2.3.4 -root-domain local -rate-limit 10 -conn-limit 5`

Testing:
Create a host entry in `/etc/hosts`:

```
127.0.2.1    foo.local
```

Then run a test container:

`docker run -P -h foo ehazlett/go-static`

Then run `curl foo.local` -- you should see "hello from go-static"

# Options

* `-conn-limit`: Connection limit (default: 0)
* `-conn-limit-var`: Variable for connection limiting (default: client.ip)
* `-docker`: TCP or Path to Docker (i.e. `unix:///var/run/docker.sock`)
* `-etcd-machines`: Comma separated list of etcd hosts (i.e. "http://127.0.0.1:4001")
* `-host-ip`: The non-local machine IP (i.e. 10.0.0.10 or 192.168.0.10, etc.)
* `-names`: Containers with names matching this regex will have upstreams created in etcd
* `-rate-limit`: Specify rate limit as requests per second (default: 0)
* `-rate-limit-burst`: Set burst rate limit (default: 1)
* `-rate-limit-var`: Variable for rate limiting (default: client.ip)
* `-root-domain`: Domain that will be used for the containers (default: `local`)
* `-root-subdomain`: Root level subdomain (i.e. www) - use this to automatically add a subdomain if using a root level domain (i.e.: `example.com` & `www.example.com`)
* `-version`: Show version

# Knowns

* I develop in containers and sometimes I have to restart Havok to get it to see the Docker events.  This does not happen when ran from the host.
