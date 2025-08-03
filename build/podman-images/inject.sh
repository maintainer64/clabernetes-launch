mkdir -p /etc/containers /var/lib/containers/storage
cat > /etc/containers/storage.conf << 'EOL'
[storage]
driver = "overlay"
graphRoot = "/var/lib/containers/storage"
runRoot = "/run/containers"
EOL
GOGC=off podman pull docker.io/alpine:3.22.1
GOGC=off podman pull ghcr.io/nokia/srlinux:25.7.1
GOGC=off podman pull registry.gitlab.com/a10869/images/vrnetlab/cisco_iol:15.4.1
GOGC=off podman pull registry.gitlab.com/a10869/images/vrnetlab/cisco_iol:15.6.3
GOGC=off podman pull registry.gitlab.com/a10869/images/vrnetlab/cisco_iol_l2:15.1.0
GOGC=off podman images