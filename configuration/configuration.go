package configuration

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type ServerConfiguration struct {
	BindHost string
}

type HostsResolverConfiguration struct {
	Source string
	Configuration map[string]string
}

type Configuration struct {
	Server ServerConfiguration
	HostResolver HostsResolverConfiguration
}

type ProxyConfiguration struct {
	To []string
}

type HostConfiguration struct {
	Domain string
	Proxy ProxyConfiguration
}

type Provider struct {

}

func (c Provider) LoadConfiguration() Configuration {
	configFile := "configuration.json"
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