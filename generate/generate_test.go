package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/livekit/livekit-server/pkg/config"
	"github.com/livekit/mediatransportutil/pkg/rtcconfig"
)

func testOpts() *ServerOptions {
	return &ServerOptions{
		Domain:     "livekit.example.com",
		TURNDomain: "livekit-turn.example.com",
		LocalRedis: true,
	}
}

func TestGenerateLiveKit(t *testing.T) {
	baseDir := t.TempDir()
	opts := testOpts()
	opts.IncludeIngress = true
	opts.WHIPDomain = "livekit-whip.example.com"

	conf, err := generateLiveKit(opts, baseDir)
	require.NoError(t, err)
	require.NotNil(t, conf)

	data, err := os.ReadFile(opts.Files.LiveKit)
	require.NoError(t, err)

	// generated config must be accepted by livekit-server's own strict parser
	parsed, err := config.NewConfig(string(data), true, nil, nil)
	require.NoError(t, err)

	require.EqualValues(t, 7880, parsed.Port)
	require.EqualValues(t, 7881, parsed.RTC.TCPPort)
	require.EqualValues(t, 50000, parsed.RTC.ICEPortRangeStart)
	require.EqualValues(t, 60000, parsed.RTC.ICEPortRangeEnd)
	require.True(t, parsed.RTC.UseExternalIP)

	require.True(t, parsed.TURN.Enabled)
	require.Equal(t, opts.TURNDomain, parsed.TURN.Domain)
	require.EqualValues(t, 5349, parsed.TURN.TLSPort)
	require.EqualValues(t, 3478, parsed.TURN.UDPPort)

	require.Len(t, parsed.Keys, 1)
	for k, s := range parsed.Keys {
		require.NotEmpty(t, k)
		require.NotEmpty(t, s)
	}

	require.Equal(t, "localhost:6379", parsed.Redis.Address)
	require.Equal(t, "rtmp://livekit.example.com:1935/x", parsed.Ingress.RTMPBaseURL)
	require.Equal(t, "https://livekit-whip.example.com/w", parsed.Ingress.WHIPBaseURL)

	// bundled redis config should be written alongside
	require.FileExists(t, opts.Files.RedisConf)
}

func TestGenerateLocal(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, generateLocal())

	data, err := os.ReadFile("livekit.yaml")
	require.NoError(t, err)

	// PortRange with no end must marshal as a plain scalar
	require.Contains(t, string(data), "udp_port: 7882")

	parsed, err := config.NewConfig(string(data), true, nil, nil)
	require.NoError(t, err)
	require.EqualValues(t, 7880, parsed.Port)
	require.EqualValues(t, 7881, parsed.RTC.TCPPort)
	require.Equal(t, rtcconfig.PortRange{Start: 7882}, parsed.RTC.UDPPort)
	require.Len(t, parsed.Keys, 1)
}

func TestGenerateEgress(t *testing.T) {
	baseDir := t.TempDir()
	opts := testOpts()
	opts.IncludeEgress = true

	lkConf, err := generateLiveKit(opts, baseDir)
	require.NoError(t, err)
	require.NoError(t, generateEgress(opts, lkConf, baseDir))

	data, err := os.ReadFile(opts.Files.Egress)
	require.NoError(t, err)

	var parsed egressConfig
	require.NoError(t, yaml.Unmarshal(data, &parsed))

	apiKey, apiSecret, err := getAPIKeySecret(lkConf)
	require.NoError(t, err)
	require.Equal(t, apiKey, parsed.ApiKey)
	require.Equal(t, apiSecret, parsed.ApiSecret)
	require.Equal(t, "wss://livekit.example.com", parsed.WsUrl)
	require.Equal(t, "localhost:6379", parsed.Redis.Address)
}

func TestGenerateEgressSkipped(t *testing.T) {
	baseDir := t.TempDir()
	opts := testOpts()

	lkConf, err := generateLiveKit(opts, baseDir)
	require.NoError(t, err)
	require.NoError(t, generateEgress(opts, lkConf, baseDir))
	require.Empty(t, opts.Files.Egress)
}

func TestGenerateIngress(t *testing.T) {
	baseDir := t.TempDir()
	opts := testOpts()
	opts.IncludeIngress = true

	lkConf, err := generateLiveKit(opts, baseDir)
	require.NoError(t, err)
	require.NoError(t, generateIngress(opts, lkConf, baseDir))

	data, err := os.ReadFile(opts.Files.Ingress)
	require.NoError(t, err)

	var parsed ingressConfig
	require.NoError(t, yaml.Unmarshal(data, &parsed))
	require.Equal(t, "wss://livekit.example.com", parsed.WsUrl)
	require.Equal(t, DefaultRTMPPort, parsed.RTMPPort)
	require.Equal(t, DefaultWHIPPort, parsed.WHIPPort)
	require.Equal(t, DefaultHTTPRelayPort, parsed.HTTPRelayPort)
	require.Equal(t, rtcconfig.PortRange{Start: DefaultRTCUDPPort}, parsed.RTCConfig.UDPPort)
	require.True(t, parsed.RTCConfig.UseExternalIP)

	// udp_port must be written as a scalar, matching what the ingress
	// service expects for a single-port config
	var raw map[string]any
	require.NoError(t, yaml.Unmarshal(data, &raw))
	rtcRaw, ok := raw["rtc_config"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, DefaultRTCUDPPort, rtcRaw["udp_port"])
}

func TestGenerateCaddy(t *testing.T) {
	baseDir := t.TempDir()
	opts := testOpts()
	opts.WHIPDomain = "livekit-whip.example.com"
	opts.ZeroSSLAPIKey = "test-api-key"

	require.NoError(t, generateCaddy(opts, baseDir))

	data, err := os.ReadFile(opts.Files.Caddy)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, yaml.Unmarshal(data, &parsed))

	content := string(data)
	require.Contains(t, content, opts.Domain)
	require.Contains(t, content, opts.TURNDomain)
	require.Contains(t, content, opts.WHIPDomain)
	require.Contains(t, content, "api_key: test-api-key")
}

func TestGenerateDocker(t *testing.T) {
	baseDir := t.TempDir()
	opts := testOpts()
	opts.IncludeEgress = true
	opts.IncludeIngress = true
	opts.ServerVersion = "latest"

	require.NoError(t, generateDocker(opts, baseDir))

	data, err := os.ReadFile(opts.Files.Docker)
	require.NoError(t, err)

	var parsed struct {
		Services map[string]struct {
			Image string `yaml:"image"`
		} `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(data, &parsed))

	for _, svc := range []string{"caddy", "livekit", "redis", "egress", "ingress"} {
		require.Contains(t, parsed.Services, svc)
	}
	require.Equal(t, "livekit/livekit-server:latest", parsed.Services["livekit"].Image)
}

func TestGenerateDockerServerOnly(t *testing.T) {
	baseDir := t.TempDir()
	opts := testOpts()
	opts.LocalRedis = false
	opts.ServerVersion = "v1.9.1"

	require.NoError(t, generateDocker(opts, baseDir))

	data, err := os.ReadFile(opts.Files.Docker)
	require.NoError(t, err)

	var parsed struct {
		Services map[string]any `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(data, &parsed))

	require.Contains(t, parsed.Services, "caddy")
	require.Contains(t, parsed.Services, "livekit")
	require.NotContains(t, parsed.Services, "redis")
	require.NotContains(t, parsed.Services, "egress")
	require.NotContains(t, parsed.Services, "ingress")
}

func TestGetAPIKeySecret(t *testing.T) {
	_, _, err := getAPIKeySecret(&config.Config{})
	require.Error(t, err)

	key, secret, err := getAPIKeySecret(&config.Config{
		Keys: map[string]string{"key": "secret"},
	})
	require.NoError(t, err)
	require.Equal(t, "key", key)
	require.Equal(t, "secret", secret)
}
