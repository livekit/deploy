package main

type Options struct {
	Domain        string
	TURNDomain    string
	ServerVersion string
	LocalRedis    bool
	CloudInit     bool

	Files ConfigFiles
}

type ConfigFiles struct {
	LiveKit string
	Caddy   string
	Docker  string
	Redis   string
}
