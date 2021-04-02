package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type CertificateLocation struct {
	Cert string
	Key string
}

type TlsServerConfiguration struct {
	Active bool
	RedirectHttp bool
	BindHost string
	BindPort int
	Certificates map[string]CertificateLocation
}

type ServerConfiguration struct {
	BindHost string
	BindPort int
	Tls TlsServerConfiguration
}

type HostsResolverConfiguration struct {
	Source string
	Configuration map[string]string
}

type Configuration struct {
	Server       ServerConfiguration
	HostResolver HostsResolverConfiguration
}

type ProxyConfiguration struct {
	To []string
}

type HostConfiguration struct {
	Domain string
	Proxy  ProxyConfiguration
}

type Provider struct {

}

func (c Provider) LoadConfiguration() Configuration {
	configFile := "/etc/kproxy/configuration.json"
	args := os.Args[1:]
	if len(args) >= 1 {
		configFile = args[0]
	}
	jsonFile, err := os.Open(configFile)
	if err != nil {
		panic(err)
	}
	byteValue, _ := ioutil.ReadAll(jsonFile)
	var configuration Configuration
	json.Unmarshal(byteValue, &configuration)
	jsonFile.Close()
	return configuration
}

func (c Provider) LoadHostsFromFile(configuration Configuration) []HostConfiguration {
	var files []string
	err := filepath.Walk(configuration.HostResolver.Configuration["path"], func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		panic(err)
	}
	hosts := make([]HostConfiguration, len(files))
	for i, file := range files {
		if !strings.HasSuffix(file, ".json") {
			continue
		}
		jsonFile, err := os.Open(file)
		if err != nil {
			panic(err)
		}
		byteValue, _ := ioutil.ReadAll(jsonFile)
		var host HostConfiguration
		json.Unmarshal(byteValue, &host)
		hosts[i] = host
		jsonFile.Close()
	}
	return hosts
}

func (c Provider) LoadHostsFromKubernetes(configuration Configuration) []HostConfiguration {
	namespace := strings.Split(configuration.HostResolver.Configuration["namespace"], ";")
	hosts := make([]HostConfiguration, 0)
	config, err := clientcmd.BuildConfigFromFlags("", configuration.HostResolver.Configuration["kubeconfig"])
	if err != nil {
		panic(err.Error())
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println("Resolving Reverse Proxy Data from Kubernetes: ")
	for _, n := range namespace {
		pods, err := client.CoreV1().Pods(n).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		addressProxyMapping := make(map[string][]string)
		for _, pod := range pods.Items {
			if _, exists := pod.Annotations["kproxy/targetDomain"]; !exists {
				continue
			}
			protocol := pod.Annotations["kproxy/sourceProtocol"]
			address := protocol + "://" + strings.Split(pod.Annotations["cni.projectcalico.org/podIP"], "/")[0]
			address += ":" + pod.Annotations["kproxy/sourcePort"]
			fmt.Println("  - " + pod.Annotations["kproxy/targetDomain"] + " > " + address)
			addressProxyMapping[pod.Annotations["kproxy/targetDomain"]] = append(addressProxyMapping[pod.Annotations["kproxy/targetDomain"]], address)
		}
		for k, v := range addressProxyMapping {
			hosts = append(hosts, HostConfiguration{
				Domain: k,
				Proxy: ProxyConfiguration {
					To: v,
				},
			})
		}
	}
	fmt.Println("done")
	return hosts
}