package main

import (
	"crypto/tls"
	"io/fs"
	"kproxy/cmd/kproxy/configuration"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type ReverseProxy struct {
	httpProxy     []*httputil.ReverseProxy
	configuration configuration.HostConfiguration
}

var domainToConfigurationMap map[string]*ReverseProxy
var certificateToDomainMap map[string]*tls.Certificate
var configurationProvider configuration.Provider
var config configuration.Configuration

func main() {
	http.HandleFunc("/", handler())
	domainToConfigurationMap = make(map[string]*ReverseProxy)
	certificateToDomainMap = make(map[string]*tls.Certificate)
	loadConfigurations()
	if config.Server.Tls.Active {
		tls := &tls.Config{}
		tls.GetCertificate = resolveCertificate()
		server := http.Server{
			Addr:      config.Server.Tls.BindHost,
			Handler:   nil,
			TLSConfig: tls,
		}
		go server.ListenAndServeTLS("", "")
	}

	err := http.ListenAndServe(config.Server.BindHost, nil)
	if err != nil {
		panic(err)
	}
}

func loadConfigurations() {
	configurationProvider = configuration.Provider{}
	config = configurationProvider.LoadConfiguration()
	if strings.ToLower(config.HostResolver.Source) == "local" {
		applyHosts(configurationProvider.LoadHostsFromFile(config), domainToConfigurationMap)
	} else if strings.ToLower(config.HostResolver.Source) == "kubernetes/calico" {
		go func() {
			for true {
				// Reload from kubernetes
				domainToConfigurationMapTemp := make(map[string]*ReverseProxy)
				applyHosts(configurationProvider.LoadHostsFromKubernetes(config), domainToConfigurationMapTemp)
				domainToConfigurationMap = domainToConfigurationMapTemp
				time.Sleep(1 * time.Minute)
			}
		}()
	}
}

func applyHosts(hosts []configuration.HostConfiguration, proxyMap map[string]*ReverseProxy) {
	for _, host := range hosts {
		proxyMap[host.Domain] = parseHostToProxy(host)
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
		var hasTls bool
		if _, exists := config.Server.Tls.Certificates[domain]; exists {
			hasTls = true
		}
		if !config.Server.Tls.Active {
			hasTls = false
		}

		if r.TLS == nil && hasTls {
			w.Header().Add("Location", "https://" + domain)
			w.WriteHeader(301)
			return
		}

		proxy.httpProxy[rand.Intn(len(proxy.httpProxy))].ServeHTTP(w, r)
	}
}

func resolveCertificate() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if _, exists := config.Server.Tls.Certificates[info.ServerName]; !exists {
			return nil, fs.ErrNotExist
		}
		if _, exists := certificateToDomainMap[info.ServerName]; !exists {
			location := config.Server.Tls.Certificates[info.ServerName]
			cert, err := tls.LoadX509KeyPair(location.Cert, location.Key)
			if err != nil {
				panic(err)
			}
			certificateToDomainMap[info.ServerName] = &cert
		}
		return certificateToDomainMap[info.ServerName], nil
	}
}