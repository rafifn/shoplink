#!/usr/bin/env bash
# One-time setup for the ShopLink VPS (Ubuntu 24.04, admin user "ubuntu").
#
# Run as root, one phase at a time, checking the result before the next:
#   sudo bash setup-server.sh base
#   sudo bash setup-server.sh swap
#   sudo bash setup-server.sh docker
#   sudo bash setup-server.sh deploy-user /path/to/shoplink_deploy.pub
#   sudo bash setup-server.sh harden     # ONLY after key login works for ubuntu AND deploy
#   sudo bash setup-server.sh firewall
#
# Every phase is safe to run again. Keep one SSH session open while running
# "harden" and "firewall" so a mistake can't lock you out.
set -euo pipefail

[[ $EUID -eq 0 ]] || { echo "Run as root (sudo)." >&2; exit 1; }
export DEBIAN_FRONTEND=noninteractive

# Packages fully updated, security updates installed automatically every day.
base() {
  apt-get update
  apt-get -y -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold upgrade
  apt-get -y install unattended-upgrades ca-certificates curl
  cat > /etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
EOF
  systemctl enable --now unattended-upgrades
  if [[ -f /var/run/reboot-required ]]; then echo "NOTE: reboot required"; fi
}

# At least ~2 GB swap so builds or memory spikes don't kill Postgres.
swap() {
  local have_mb
  have_mb=$(free -m | awk '/^Swap:/ {print $2}')
  if (( have_mb >= 1800 )); then
    echo "Swap already ${have_mb} MB, not adding more."
  else
    fallocate -l 2G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
  fi
  echo 'vm.swappiness=10' > /etc/sysctl.d/99-shoplink-swap.conf
  sysctl -q -p /etc/sysctl.d/99-shoplink-swap.conf
}

# Docker Engine + Compose plugin from Docker's official apt repository.
install_docker() {
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  # shellcheck source=/dev/null
  . /etc/os-release
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${UBUNTU_CODENAME} stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update
  apt-get -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  # Cap container logs so they can't fill the 40 GB disk.
  cat > /etc/docker/daemon.json <<'EOF'
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" }
}
EOF
  systemctl enable docker
  systemctl restart docker
}

# "deploy": key-only user CI logs in as. In the docker group (which is
# effectively root on this box), no sudo, no password.
deploy_user() {
  local pub=${1:?usage: deploy-user /path/to/deploy_key.pub}
  id deploy &>/dev/null || adduser --disabled-password --gecos "" deploy
  usermod -aG docker deploy
  install -d -m 700 -o deploy -g deploy /home/deploy/.ssh
  install -m 600 -o deploy -g deploy "$pub" /home/deploy/.ssh/authorized_keys
  install -d -m 750 -o deploy -g deploy /opt/shoplink
}

# SSH: keys only, no root login. The 00- prefix makes these win over
# 50-cloud-init.conf, because sshd keeps the first value it reads.
harden() {
  cat > /etc/ssh/sshd_config.d/00-shoplink-hardening.conf <<'EOF'
PermitRootLogin no
PasswordAuthentication no
KbdInteractiveAuthentication no
PubkeyAuthentication yes
MaxAuthTries 3
X11Forwarding no
EOF
  sshd -t
  systemctl reload ssh
}

# Firewall: only SSH, HTTP, HTTPS (incl. HTTP/3 over UDP) come in.
# Docker-published ports bypass ufw, so docker-compose.prod.yml must publish
# only 80/443 (Caddy) and never the database port.
firewall() {
  apt-get -y install ufw
  ufw default deny incoming
  ufw default allow outgoing
  ufw allow 22/tcp
  ufw allow 80/tcp
  ufw allow 443/tcp
  ufw allow 443/udp
  ufw --force enable
  ufw status verbose
}

case "${1:-}" in
  base) base ;;
  swap) swap ;;
  docker) install_docker ;;
  deploy-user) deploy_user "${2:-}" ;;
  harden) harden ;;
  firewall) firewall ;;
  *) echo "usage: $0 {base|swap|docker|deploy-user <pubkey>|harden|firewall}" >&2; exit 1 ;;
esac
