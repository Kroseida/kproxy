package configuration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type ProxyConfiguration struct {
	To []string
}

type HostConfiguration struct {
	Domain string
	Proxy ProxyConfiguration
}

type FileHostConfigurationProvider struct {
}

func (c FileHostConfigurationProvider) LoadHostsFromConfiguration() []HostConfiguration {
	var files []string
	err := filepath.Walk("hosts", func(path string, info os.FileInfo, err error) error {
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
			fmt.Println(err)
		}
		byteValue, _ := ioutil.ReadAll(jsonFile)
		var host HostConfiguration
		json.Unmarshal(byteValue, &host)
		hosts[i] = host
		jsonFile.Close()
	}
	return hosts
}