package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/ryancurrah/gomodbump"
	"gopkg.in/yaml.v2"
)

var (
	configFile = ".gomodbump.yaml"
	logger     = log.New(os.Stderr, "", 0)
)

func main() {
	config, err := getConfig()
	if err != nil {
		logger.Fatal(err)
	}

	config.SCM.BitbucketServer.Username = os.Getenv("BITBUCKET_SERVER_USERNAME")
	config.SCM.BitbucketServer.Password = os.Getenv("BITBUCKET_SERVER_PASSWORD")
	config.VCS.Git.Username = os.Getenv("GIT_USERNAME")
	config.VCS.Git.Password = os.Getenv("GIT_PASSWORD")

	bumper := gomodbump.NewGoModBump(*config)

	err = bumper.Run()
	if err != nil {
		log.Fatalf("running gomodbump failed: %s", err)
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}

func getConfig() (*gomodbump.Configuration, error) {
	config := gomodbump.Configuration{}

	home, err := homedir.Dir()
	if err != nil {
		return nil, fmt.Errorf("unable to find home directory, %s", err)
	}

	cfgFile := ""
	homeDirCfgFile := filepath.Join(home, configFile)

	switch {
	case fileExists(configFile):
		cfgFile = configFile
	case fileExists(homeDirCfgFile):
		cfgFile = homeDirCfgFile
	default:
		return nil, fmt.Errorf("could not find config file in %s, %s", configFile, homeDirCfgFile)
	}

	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %s", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("could not parse config file: %s", err)
	}

	return &config, nil
}
