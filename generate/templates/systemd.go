package templates

const SystemdService = `[Unit]
Description=LiveKit Server Container
After=docker.service
Requires=docker.service

[Service]
Restart=always
WorkingDirectory=/opt/livekit
# Shutdown container (if running) when unit is started
ExecStartPre=docker-compose -f docker-compose.yaml down
ExecStart=docker-compose -f docker-compose.yaml up
ExecStop=docker-compose -f docker-compose.yaml down

[Install]
WantedBy=multi-user.target
`
