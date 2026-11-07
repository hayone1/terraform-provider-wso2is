# A single Hetzner Cloud server running k3s, with DNS records for the lab domain.
# WSO2 Identity Server itself is deployed afterwards with Kubernetes manifests
# (deploy/k8s), see deploy/README.md and `task lab:up`.

locals {
  labels = {
    "app.kubernetes.io/part-of" = "wso2is-lab"
    "managed-by"                = "terraform"
  }
  lab_domain = "${var.dns_subdomain}.${var.dns_zone}"
}

resource "hcloud_ssh_key" "admin" {
  name       = "${var.name}-admin"
  public_key = var.ssh_public_key
  labels     = local.labels
}

resource "hcloud_firewall" "lab" {
  name   = var.name
  labels = local.labels

  rule {
    description = "SSH (also carries the tunnel to the Kubernetes API)"
    direction   = "in"
    protocol    = "tcp"
    port        = "22"
    source_ips  = var.ssh_allowed_cidrs
  }

  rule {
    description = "HTTP (Let's Encrypt HTTP-01 challenges)"
    direction   = "in"
    protocol    = "tcp"
    port        = "80"
    source_ips  = ["0.0.0.0/0", "::/0"]
  }

  rule {
    description = "HTTPS (WSO2 Identity Server through Traefik)"
    direction   = "in"
    protocol    = "tcp"
    port        = "443"
    source_ips  = ["0.0.0.0/0", "::/0"]
  }

  rule {
    description = "ICMP (ping and path MTU discovery)"
    direction   = "in"
    protocol    = "icmp"
    source_ips  = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_server" "lab" {
  name         = var.name
  server_type  = var.server_type
  location     = var.location
  image        = var.image
  ssh_keys     = [hcloud_ssh_key.admin.id]
  firewall_ids = [hcloud_firewall.lab.id]
  labels       = local.labels

  user_data = templatefile("${path.module}/cloud-init.yaml.tftpl", {
    k3s_version = var.k3s_version
    lab_domain  = local.lab_domain
  })

  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
  }

  lifecycle {
    # Changing cloud-init or the image would rebuild the server and lose its data.
    # Run `task lab:destroy` and `task lab:up` to start over on purpose.
    ignore_changes = [user_data, image]
  }
}

# <subdomain>.<zone> and *.<subdomain>.<zone> point at the server, so every
# Ingress host below the lab domain works without further DNS changes.
resource "hcloud_zone_rrset" "a" {
  for_each = toset([var.dns_subdomain, "*.${var.dns_subdomain}"])

  zone    = var.dns_zone
  name    = each.value
  type    = "A"
  ttl     = 300
  records = [{ value = hcloud_server.lab.ipv4_address }]
  labels  = local.labels
}

resource "hcloud_zone_rrset" "aaaa" {
  for_each = toset([var.dns_subdomain, "*.${var.dns_subdomain}"])

  zone    = var.dns_zone
  name    = each.value
  type    = "AAAA"
  ttl     = 300
  records = [{ value = hcloud_server.lab.ipv6_address }]
  labels  = local.labels
}
