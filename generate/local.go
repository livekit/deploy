package main

import (
	"fmt"
	"os"
	"time"

	"github.com/livekit/livekit-server/pkg/config"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/utils"
	"gopkg.in/yaml.v3"
)

func generateLocal() error {
	apiKey := utils.NewGuid(utils.APIKeyPrefix)
	apiSecret := utils.RandomSecret()
	conf := config.Config{
		Keys: map[string]string{
			apiKey: apiSecret,
		},
		Logging: config.LoggingConfig{
			JSON:  false,
			Level: "info",
		},
		Port: 7880,
		RTC: config.RTCConfig{
			TCPPort:       7881,
			UDPPort:       7882,
			UseExternalIP: false,
		},
	}

	out, err := os.Create(outputPath("livekit.yaml"))
	if err != nil {
		return err
	}
	defer out.Close()

	data, err := yaml.Marshal(&conf)
	if err != nil {
		return err
	}
	err = os.WriteFile(outputPath("livekit.yaml"), data, filePerms)
	if err != nil {
		return err
	}

	// get local ip
	ips, err := config.GetLocalIPAddresses()
	if err != nil {
		return err
	}

	ip := "127.0.0.1"
	if !isDocker() && len(ips) > 0 {
		ip = ips[0]
	}

	// generate token
	token := auth.NewAccessToken(apiKey, apiSecret)
	token.SetIdentity("tony_stark")
	token.SetName("Tony Stark")
	token.AddGrant(&auth.VideoGrant{
		Room:     "stark-tower",
		RoomJoin: true,
	})
	token.SetValidFor(10000 * time.Hour)
	jwt, err := token.ToJWT()
	if err != nil {
		return err
	}

	fmt.Println("Generated livekit.yaml that's suitable for local testing")
	fmt.Println("API Key: " + apiKey)
	fmt.Println("API Secret: " + apiSecret)
	fmt.Println()
	fmt.Println("Start LiveKit with:")
	fmt.Println("docker run --rm \\")
	fmt.Println("    -p 7880:7880 \\")
	fmt.Println("    -p 7881:7881 \\")
	fmt.Println("    -p 7882:7882/udp \\")
	fmt.Println("    -v $PWD/livekit.yaml:/livekit.yaml \\")
	fmt.Println("    livekit/livekit-server \\")
	fmt.Println("    --config /livekit.yaml \\")
	fmt.Println("    --node-ip=" + ip)
	fmt.Println()
	if isDocker() {
		fmt.Println("Note: --node-ip needs to be reachable by the client. 127.0.0.1 is accessible only to the current machine")
		fmt.Println()
	}

	fmt.Println("Here's a test token generated with your keys: " + jwt)
	fmt.Println()
	fmt.Println("Access tokens identifies the participant as well as the room it's connecting to")
	return nil
}
