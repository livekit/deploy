package main

import (
	"github.com/livekit/deploy/generate/templates"
)

type StartupScriptKind string

const (
	StartupScriptNone            StartupScriptKind = "no"
	StartupScriptCloudInitAmazon StartupScriptKind = "cloud_init.amazon.yaml"
	StartupScriptCloudInitUbuntu StartupScriptKind = "cloud_init.ubuntu.yaml"
	StartupScriptShellScript     StartupScriptKind = "init_script.sh"
)

func (k StartupScriptKind) Description() string {
	switch k {
	case StartupScriptCloudInitAmazon:
		return "Cloud Init for Amazon Linux"
	case StartupScriptCloudInitUbuntu:
		return "Cloud Init for Ubuntu"
	case StartupScriptShellScript:
		return "Startup Shell Script"
	default:
		return "Skip"
	}
}

func (k StartupScriptKind) Template() string {
	switch k {
	case StartupScriptCloudInitAmazon:
		return templates.CloudInitAmazon2Template
	case StartupScriptCloudInitUbuntu:
		return templates.CloudInitUbuntuTemplate
	case StartupScriptShellScript:
		return templates.StartupScriptTemplate
	default:
		return ""
	}
}

func CloudInitFromDescription(str string) StartupScriptKind {
	switch str {
	case StartupScriptCloudInitAmazon.Description():
		return StartupScriptCloudInitAmazon
	case StartupScriptCloudInitUbuntu.Description():
		return StartupScriptCloudInitUbuntu
	case StartupScriptShellScript.Description():
		return StartupScriptShellScript
	default:
		return StartupScriptNone
	}
}

type Options struct {
	Domain        string
	TURNDomain    string
	ServerVersion string
	LocalRedis    bool
	CloudInit     StartupScriptKind

	Files ConfigFiles
}

type ConfigFiles struct {
	LiveKit   string
	Caddy     string
	Docker    string
	RedisConf string
}
