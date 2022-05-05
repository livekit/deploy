package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"text/template"

	"github.com/google/go-github/v42/github"
	"github.com/livekit/deploy/generate/templates"
	"github.com/livekit/livekit-server/pkg/config"
	"github.com/livekit/protocol/utils"
	"github.com/manifoldco/promptui"
	"gopkg.in/yaml.v3"
)

var (
	domainRegexp  = regexp.MustCompile(`^(?:[_A-Za-z0-9](?:[_A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)+(?:[A-Za-z](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)?$`)
	versionRegexp = regexp.MustCompile(`^v[0-9]+(\.[0-9]+){0,2}$`)
)

func generateProduction() error {
	fmt.Println("Generating config for production LiveKit deployment")
	fmt.Println("This deployment will utilize docker-compose and Caddy. It'll set up a secure LiveKit installation with built-in TURN/TLS")
	fmt.Println("SSL Certificates for HTTPS and TURN/TLS will be generated automatically via Lets Encrypt.")
	fmt.Println()
	opts := Options{}
	var err error
	prompt := promptui.Prompt{
		Label:    "Primary domain name (i.e. livekit.myhost.com)",
		Validate: validateDomain,
		Stdout:   BellSkipper,
	}
	if opts.Domain, err = prompt.Run(); err != nil {
		return err
	}

	// early termination here for test. TODO: move towards the end
	baseDir := outputPath(opts.Domain)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}

	prompt = promptui.Prompt{
		Label: "TURN domain name (i.e. livekit-turn.myhost.com)",
		Validate: func(s string) error {
			if err := validateDomain(s); err != nil {
				return err
			}
			if s == opts.Domain {
				return fmt.Errorf("cannot be same as primary domain name")
			}
			return nil
		},
		Stdout: BellSkipper,
	}
	if opts.TURNDomain, err = prompt.Run(); err != nil {
		return err
	}

	// version
	version, err := getLatestVersion()
	if err != nil {
		return err
	}
	versionPrompt := promptui.SelectWithAdd{
		Label:    "LiveKit version",
		Items:    []string{"latest", version},
		AddLabel: "custom",
		Validate: validateVersion,
	}
	_, opts.ServerVersion, err = versionPrompt.Run()
	if err != nil {
		return err
	}

	// redis
	redisPrompt := promptui.Select{
		Label: "Use external Redis",
		Items: []string{
			"no - (we'll bundle Redis)",
			"yes",
		},
		Stdout: BellSkipper,
	}
	idx, _, err := redisPrompt.Run()
	if err != nil {
		return err
	}
	if idx == 0 {
		opts.LocalRedis = true
	}

	startupScripts := []StartupScriptKind{
		StartupScriptShellScript,
		StartupScriptCloudInitAmazon,
		StartupScriptCloudInitUbuntu,
		StartupScriptNone,
	}
	var descriptions []string

	for _, s := range startupScripts {
		descriptions = append(descriptions, s.Description())
	}

	// cloud init
	cloudPrompt := promptui.Select{
		Label:  "Generate a startup script? It'll write configuration files to the right spots on the server.",
		Items:  descriptions,
		Stdout: BellSkipper,
	}
	idx, _, err = cloudPrompt.Run()
	if err != nil {
		return err
	}
	opts.CloudInit = startupScripts[idx]

	// generate files
	conf, err := generateLiveKit(&opts, baseDir)
	if err != nil {
		return err
	}
	if err = generateCaddy(&opts, baseDir); err != nil {
		return err
	}
	if err = generateDocker(&opts, baseDir); err != nil {
		return err
	}

	if opts.CloudInit != StartupScriptNone {
		if err = generateStartupScript(&opts, baseDir); err != nil {
			return err
		}
	}

	return printInstructions(&opts, conf)
}

func printInstructions(opts *Options, conf *config.Config) error {
	fmt.Println("Your production config files are generated in directory:", opts.Domain)
	fmt.Println()
	fmt.Printf("Please point DNS for %s and %s to the IP address of your server.\n", opts.Domain, opts.TURNDomain)
	fmt.Println("Once started, Caddy will automatically acquire TLS certificates for the domains.")
	fmt.Println()
	if opts.CloudInit != StartupScriptNone {
		fmt.Printf("The file \"%s\" is a script that can be used in the \"user-data\" field when starting a new VM.\n",
			string(opts.CloudInit))
	} else {
		fmt.Println("You can copy the folder to your server and run: \"docker-compose up\"")
	}
	fmt.Println()

	fmt.Println("Please ensure the following ports are accessible on the server")
	fmt.Println(" * 443 - primary HTTPS and TURN/TLS")
	fmt.Println(" * 80 - for TLS issuance")
	fmt.Printf(" * %d - for WebRTC over TCP\n", conf.RTC.TCPPort)
	fmt.Println(" * 443/UDP - for TURN/UDP")
	fmt.Printf(" * %d-%d/UDP - for WebRTC over UDP\n", conf.RTC.ICEPortRangeStart, conf.RTC.ICEPortRangeEnd)
	fmt.Println()
	fmt.Printf("Server URL: wss://%s\n", opts.Domain)
	var apiKey, apiSecret string
	for k, s := range conf.Keys {
		apiKey = k
		apiSecret = s
	}
	return printKeysAndToken(apiKey, apiSecret)
}

func validateDomain(domain string) error {
	if domainRegexp.MatchString(domain) {
		return nil
	}
	return fmt.Errorf("requires a valid domain name")
}

func validateVersion(version string) error {
	if versionRegexp.MatchString(version) {
		return nil
	}
	return fmt.Errorf("not a valid version number (i.e. v0.15)")
}

func getLatestVersion() (string, error) {
	client := github.NewClient(nil)
	release, _, err := client.Repositories.GetLatestRelease(context.Background(), "livekit", "livekit-server")
	if err != nil {
		return "", err
	}
	return release.GetTagName(), nil
}

func generateLiveKit(opts *Options, baseDir string) (*config.Config, error) {
	apiKey := utils.NewGuid(utils.APIKeyPrefix)
	apiSecret := utils.RandomSecret()
	conf := config.Config{
		Keys: map[string]string{
			apiKey: apiSecret,
		},
		Logging: config.LoggingConfig{
			JSON: false,
		},
		RTC: config.RTCConfig{
			UseExternalIP:     true,
			TCPPort:           7881,
			ICEPortRangeStart: 50000,
			ICEPortRangeEnd:   60000,
		},
		Port: 7880,
		TURN: config.TURNConfig{
			Enabled:     true,
			Domain:      opts.TURNDomain,
			ExternalTLS: true,
			TLSPort:     5349,
			UDPPort:     443,
		},
	}
	if opts.LocalRedis {
		conf.Redis = config.RedisConfig{
			Address: "localhost:6379",
		}
		// copy redis over to basedir
		opts.Files.RedisConf = path.Join(baseDir, "redis.conf")
		if err := os.WriteFile(opts.Files.RedisConf, []byte(templates.RedisConf), filePerms); err != nil {
			return nil, err
		}
	} else {
		conf.Redis = config.RedisConfig{
			Address: "<redis-host>:6379",
		}
	}

	// write config
	data, err := yaml.Marshal(&conf)
	if err != nil {
		return nil, err
	}
	opts.Files.LiveKit = path.Join(baseDir, "livekit.yaml")
	return &conf, os.WriteFile(opts.Files.LiveKit, data, filePerms)
}

func generateCaddy(opts *Options, baseDir string) error {
	tmpl, err := template.New("caddy").Parse(templates.CaddyConfigTemplate)
	if err != nil {
		return err
	}
	opts.Files.Caddy = path.Join(baseDir, "caddy.yaml")
	f, err := os.Create(opts.Files.Caddy)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, opts)
}

func generateDocker(opts *Options, baseDir string) error {
	tmpl, err := template.New("docker").Parse(templates.DockerComposeBaseTemplate)
	if err != nil {
		return err
	}
	buf := bytes.Buffer{}
	if err := tmpl.Execute(&buf, opts); err != nil {
		return err
	}

	if opts.LocalRedis {
		buf.WriteString(templates.DockerComposeRedis)
	}
	opts.Files.Docker = path.Join(baseDir, "docker-compose.yaml")
	return os.WriteFile(opts.Files.Docker, buf.Bytes(), filePerms)
}

func readAndPrefix(filePath string, prefix string) (string, error) {
	body, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return prefixLines(string(body), prefix), nil
}

func prefixLines(input string, prefix string) string {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	output := ""
	for _, line := range lines {
		output += prefix + line + "\n"
	}
	return output
}
