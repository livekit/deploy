package templates

const CloudInitAmazon2Template = `#cloud-config
# This file is used as a user-data script to start a VM
# It'll upload configs to the right location and install LiveKit as a systemd service
# LiveKit will be started automatically at machine startup
repo_update: true
repo_upgrade: all

packages:
  - docker

bootcmd:
  - mkdir -p /opt/livekit/caddy_data

write_files:
  - path: /opt/livekit/livekit.yaml
    content: |
{{.LiveKitConfig}}
  - path: /opt/livekit/caddy.yaml
    content: |
{{.CaddyConfig}}
  - path: /opt/livekit/docker-compose.yaml
    content: |
{{.DockerComposeConfig}}
  - path: /etc/systemd/system/livekit-docker.service
    content: |
{{.SystemService}}
{{- if .RedisConf }}
  - path: /opt/livekit/redis.conf
    content: |
{{.RedisConf}}
{{- end }}

runcmd:
  - curl -L "https://github.com/docker/compose/releases/download/v2.2.3/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
  - chmod 755 /usr/local/bin/docker-compose
  - systemctl enable docker
  - systemctl start docker
  - systemctl enable livekit-docker
  - systemctl start livekit-docker
`
