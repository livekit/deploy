package main

import (
	"fmt"
	"os"
	"path"

	"gopkg.in/yaml.v3"

	"github.com/livekit/livekit-server/pkg/config"
	"github.com/livekit/protocol/redis"
)

const DefaultRTMPPort = 1935

// duplicate of livekit/ingress/pkg/config/config.go
// avoid importing the entire package during build
type ingressConfig struct {
	Redis     *redis.RedisConfig `yaml:"redis"`
	ApiKey    string             `yaml:"api_key"`
	ApiSecret string             `yaml:"api_secret"`
	WsUrl     string             `yaml:"ws_url"`
	RTMPPort  int                `yaml:"rtmp_port"`
}

func generateIngress(opts *ServerOptions, lkConf *config.Config, baseDir string) error {
	if !opts.IncludeIngress {
		return nil
	}

	ingressConf := &ingressConfig{}
	apiKey, apiSecret, err := getAPIKeySecret(lkConf)
	if err != nil {
		return err
	}
	ingressConf.ApiKey = apiKey
	ingressConf.ApiSecret = apiSecret
	ingressConf.WsUrl = fmt.Sprintf("wss://%s", opts.Domain)
	ingressConf.Redis = opts.RedisConfig()
	ingressConf.RTMPPort = DefaultRTMPPort

	// write config
	data, err := yaml.Marshal(ingressConf)
	if err != nil {
		return err
	}
	opts.Files.Ingress = path.Join(baseDir, "ingress.yaml")
	return os.WriteFile(opts.Files.Ingress, data, filePerms)
}
