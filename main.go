package main

import (
	"kproxy/configuration"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type ReverseProxy struct {
	httpProxy     []*httputil.ReverseProxy
	configuration configuration.HostConfiguration
}

var domainToConfigurationMap map[string]*ReverseProxy
var configurationProvider configuration.FileHostConfigurationProvider

func main() {
	http.HandleFunc("/", handler())

	domainToConfigurationMap = make(map[string]*ReverseProxy)
	loadConfigurations()
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		panic(err)
	}
}

func loadConfigurations() {
	configurationProvider := configuration.FileHostConfigurationProvider{}
	// Json File Configuration /hosts/default.json
	applyHosts(configurationProvider.LoadHostsFromConfiguration())
}

func applyHosts(hosts []configuration.HostConfiguration) {
	for _, host := range hosts {
		domainToConfigurationMap[host.Domain] = parseHostToProxy(host)
	}
}

func parseHostToProxy(configuration configuration.HostConfiguration) *ReverseProxy {
	proxy := make([]*httputil.ReverseProxy, len(configuration.Proxy.To))
	for x, origin := range configuration.Proxy.To {
		remote, err := url.Parse(origin)
		if err != nil {
			panic(err)
		}
		proxy[x] = httputil.NewSingleHostReverseProxy(remote)
	}
	return &ReverseProxy{
		httpProxy:     proxy,
		configuration: configuration,
	}
}

func handler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := strings.Split(r.Host, ":")[0]
		proxy := domainToConfigurationMap[domain]
		if proxy == nil {
			return
		}
		proxy.httpProxy[rand.Intn(len(proxy.httpProxy))].ServeHTTP(w, r)
	}
}
