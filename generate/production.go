package main

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/go-github/v42/github"
	"github.com/manifoldco/promptui"
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
