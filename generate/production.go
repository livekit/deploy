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
	domainRegexp  = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]{1,62}[A-Za-z0-9]\.)+[A-Za-z]{2,6}$`)
	versionRegexp = regexp.MustCompile(`^v[0-9]+(\.[0-9]+){0,2}$`)
)

func generateProduction() error {
	fmt.Println("Generating config for production LiveKit deployment")
	fmt.Println("This deployment will utilize docker-compose and Caddy. It'll be secured automatically with Caddy's built-in TLS")
	fmt.Println()
	opts := Options{}
	var err error
	prompt := promptui.Prompt{
		Label:    "Primary domain name",
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
		Label: "TURN domain name",
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
		Items:    []string{version},
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
			"no - Redis will be included in the setup",
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

	// cloud init
	cloudPrompt := promptui.Select{
		Label: "Generate cloud-init?",
		Items: []CloudInitKind{
			CloudInitNo,
			CloudInitAmazon,
			CloudInitUbuntu,
		},
		Stdout: BellSkipper,
	}
	_, cloudKind, err := cloudPrompt.Run()
	if err != nil {
		return err
	}
	opts.CloudInit = CloudInitKind(cloudKind)

	// generate files
	if err = generateLiveKit(&opts, baseDir); err != nil {
		return err
	}
	if err = generateCaddy(&opts, baseDir); err != nil {
		return err
	}
	if err = generateDocker(&opts, baseDir); err != nil {
		return err
	}

	if opts.CloudInit != CloudInitNo {
		if err = generateCloudInit(&opts, baseDir); err != nil {
			return err
		}
	}

	return nil
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

func generateLiveKit(opts *Options, baseDir string) error {
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
	}
	if opts.LocalRedis {
		conf.Redis = config.RedisConfig{
			Address: "localhost:6379",
		}
		// copy redis over to basedir
		opts.Files.RedisConf = path.Join(baseDir, "redis.conf")
		if err := os.WriteFile(opts.Files.RedisConf, []byte(templates.RedisConf), filePerms); err != nil {
			return err
		}
	} else {
		conf.Redis = config.RedisConfig{
			Address: "<redis-host>:6379",
		}
	}

	// write config
	data, err := yaml.Marshal(&conf)
	if err != nil {
		return err
	}
	opts.Files.LiveKit = path.Join(baseDir, "livekit.yaml")
	return os.WriteFile(opts.Files.LiveKit, data, filePerms)
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

type cloudInitContent struct {
	LiveKitConfig       string
	CaddyConfig         string
	DockerComposeConfig string
	SystemService       string
	RedisConf           string
}

func generateCloudInit(opts *Options, baseDir string) error {
	if opts.CloudInit == CloudInitNo {
		return nil
	}

	// prep files
	var err error
	content := cloudInitContent{}
	// six space indent for yaml
	indent := "      "
	if content.LiveKitConfig, err = readAndPrefix(opts.Files.LiveKit, indent); err != nil {
		return err
	}
	if content.CaddyConfig, err = readAndPrefix(opts.Files.Caddy, indent); err != nil {
		return err
	}
	if content.DockerComposeConfig, err = readAndPrefix(opts.Files.Docker, indent); err != nil {
		return err
	}
	if opts.LocalRedis {
		if content.RedisConf, err = readAndPrefix(opts.Files.RedisConf, indent); err != nil {
			return err
		}
	}
	// system service
	content.SystemService = prefixLines(templates.SystemdService, indent)

	tmpl, err := template.New("cloud-init").Parse(templates.CloudInitUbuntuTemplate)
	if err != nil {
		return err
	}

	target := path.Join(baseDir, fmt.Sprintf("cloud-init.%s.yaml", opts.CloudInit))
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, &content)
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
