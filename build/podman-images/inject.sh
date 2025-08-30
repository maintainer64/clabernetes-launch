mkdir -p /etc/containers /var/lib/containers/storage
cat > /etc/containers/storage.conf << 'EOL'
[storage]
driver = "overlay"
graphRoot = "/var/lib/containers/storage"
runRoot = "/run/containers"
EOL
GOGC=off podman pull docker.io/alpine:3.23.4
GOGC=off podman pull ghcr.io/nokia/srlinux:26.3.1
GOGC=off podman images