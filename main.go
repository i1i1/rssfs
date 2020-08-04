package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

type Feed struct {
	URL string `hcl:"url"`
}

type Category struct {
	// A category has a name in its title and zero or more feeds.
	Name  string  `hcl:"name,label"`
	Feeds []*Feed `hcl:"feed,block"`
}

type RssfsConfig struct {
	MountPoint string      `hcl:"mountpoint"`
	Categories []*Category `hcl:"category,block"`
}

var config = ReadConfig(ConfigFilePath())

func ConfigFilePath() string {
	path := ""
	if runtime.GOOS == "darwin" {
		path = fmt.Sprintf("%s/Library/Application Support", os.Getenv("HOME"))
	} else {
		path = os.Getenv("XDG_CONFIG_HOME")
	}
	return fmt.Sprintf("%s/rssfs.hcl", path)
}

func die(err error) {
	if err != nil {
		panic(err)
	}
}

func ReadConfig(path string) (cfg RssfsConfig) {
	die(hclsimple.DecodeFile(path, nil, &cfg))
	return
}

func main() {
	Mount(config)
}
