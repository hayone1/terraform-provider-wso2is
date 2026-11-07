output "ipv4_address" {
  description = "Public IPv4 address of the server."
  value       = hcloud_server.lab.ipv4_address
}

output "ipv6_address" {
  description = "Public IPv6 address of the server."
  value       = hcloud_server.lab.ipv6_address
}

output "lab_domain" {
  description = "Domain whose names all resolve to the server."
  value       = local.lab_domain
}
