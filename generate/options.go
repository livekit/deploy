package main

type CloudInitKind string

const (
	CloudInitNo     CloudInitKind = "no"
	CloudInitAmazon CloudInitKind = "amzn2"
	CloudInitUbuntu CloudInitKind = "ubuntu"
)

type Options struct {
	Domain        string
	TURNDomain    string
	ServerVersion string
	LocalRedis    bool
	CloudInit     CloudInitKind

	Files ConfigFiles
}

type ConfigFiles struct {
	LiveKit   string
	Caddy     string
	Docker    string
	RedisConf string
}
