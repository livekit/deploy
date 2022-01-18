package templates

const DockerComposeBaseTemplate = `# LiveKit requires host networking, which is only available on Linux
# This compose will not function correctly on Mac or Windows
version: "3.9"
services:
  caddy:
    image: livekit/caddyl4
    command: run --config /etc/caddy.yaml --adapter yaml
    restart: unless-stopped
    network_mode: "host"
    volumes:
      - $PWD/caddy.yaml:/etc/caddy.yaml
      - $PWD/caddy_data:/data
  livekit:
    image: livekit/livekit-server:{{.ServerVersion}}
    command: --config /etc/livekit.yaml
    restart: unless-stopped
    network_mode: "host"
    volumes:
      - $PWD/livekit.yaml:/etc/livekit.yaml
`

const DockerComposeRedis = `  redis:
    image: redis:6-alpine
    command: redis-server /etc/redis.conf
    network_mode: "host"
    volumes:
      - $PWD/redis.conf:/etc/redis.conf
`
