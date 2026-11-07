# Run WSO2 Identity Server for the provider

This folder gives you two ways to run a WSO2 Identity Server that the provider and its acceptance
tests can use:

* **Docker** runs a throw-away server on your own computer. It is the quickest option, and it is
  all that a contributor needs.
* **The Hetzner lab** runs a server that is closer to production. It uses Kubernetes (k3s),
  PostgreSQL, a public host name and a Let's Encrypt certificate. The maintainers use it for
  the full test runs.

## Docker (local)

```shell
task wso2:up                      # WSO2 IS 7.3.0 on https://localhost:9443 (admin/admin)
task wso2:up WSO2_VERSION=7.2.0   # another version
task wso2:logs                    # follow the server log
task wso2:down                    # remove the server
```

`task wso2:up` starts the official image with its built-in H2 database. It adds
[`docker/deployment-overrides.toml`](docker/deployment-overrides.toml) to the server's
configuration. These overrides change two settings that the acceptance tests need:

* `[tenant_mgt] tenant_deletion = true` lets `wso2is_tenant` delete tenants.
* An access control rule allows `DELETE /api/server/v1/flow`, so that `wso2is_flow` can delete
  flows. WSO2 IS 7.3.0 ships without this rule.

## The Hetzner lab

The lab is one Hetzner Cloud server that runs k3s. Terraform (or OpenTofu) creates the server, its
firewall and its DNS records. Kubernetes manifests then install WSO2 Identity Server on it.

```
             Internet
                 │  https://wso2is.lab.<zone>
                 ▼
 ┌──────────── Hetzner server (k3s) ─────────────┐
 │  Traefik  ── Let's Encrypt certificate        │
 │     │        (cert-manager, HTTP-01)          │
 │     ▼  HTTPS, checked against an internal CA  │
 │  WSO2 Identity Server 7.3.0 ── PostgreSQL 17  │
 │     └── Mailpit (catches all mail)            │
 └───────────────────────────────────────────────┘
```

### What you need

* A Hetzner Cloud project with an API token that can read and write.
* A DNS zone in the same project (Hetzner DNS). The lab adds `<subdomain>.<zone>` and
  `*.<subdomain>.<zone>` records to it.
* An SSH key pair on your computer.
* Terraform or OpenTofu, `kubectl`, `envsubst` and `curl`.

### Set up `.env`

Copy `example.env` to `.env` if you have not done so, and fill in the lab section:

| Setting | Meaning |
|---------|---------|
| `HCLOUD_TOKEN` | The Hetzner Cloud API token. |
| `LAB_DNS_ZONE` | The DNS zone, for example `example.com`. |
| `LAB_SUBDOMAIN` | The lab's subdomain, for example `lab`. |
| `LAB_SSH_KEY` | Your private SSH key. The `.pub` file must be next to it. The default is `~/.ssh/id_ed25519`. |
| `WSO2_HOSTNAME` | The server's public name. It must be below the lab domain, for example `wso2is.lab.example.com`. |
| `WSO2_ADMIN_*`, `WSO2_DB_*`, ... | Passwords for the server, the database and the keystore. Choose strong values. |

### Start the lab

```shell
task lab:up
```

This one command runs these steps in order:

1. `task lab:infra` creates the server, the firewall and the DNS records with Terraform. Then it
   waits until cloud-init has installed k3s. Add `TF_CLI=tofu` to use OpenTofu.
2. `task lab:platform` installs cert-manager and the Let's Encrypt issuers.
3. `task lab:deploy` creates the Secret with your passwords, and applies the WSO2 Identity Server,
   PostgreSQL and Mailpit manifests. Then it waits for the rollout.
4. `task lab:wait` waits until `https://$WSO2_HOSTNAME` answers with a trusted certificate.

The first run takes about ten minutes. `kubectl` reaches the cluster through an SSH tunnel on
`localhost:16443`, which `task lab:tunnel` opens. The Kubernetes API is never open to the internet.

### Use the lab

```shell
task testacc -- ./...         # run the acceptance tests against the lab
task lab:status               # pods, certificates and ingress
task lab:logs -- --since=10m  # the WSO2 log
task lab:mailpit              # mail that WSO2 sent, on http://localhost:8025
task lab:ssh -- uptime        # run a command on the server
task lab:restart              # restart WSO2 and keep the data
task lab:reset                # delete the database and start again (asks first)
task lab:destroy              # delete the server and everything on it (asks first)
```

The server costs money while it exists. Hetzner bills by the hour, so run `task lab:destroy` when
you do not need the lab.

### How the pieces fit together

* [`hetzner/`](hetzner) is a Terraform configuration. It creates the server (Ubuntu 24.04 and
  k3s, installed by [cloud-init](hetzner/cloud-init.yaml.tftpl)), a firewall that only opens
  ports 22, 80 and 443, and DNS records for the lab domain and every name below it.
* [`k8s/base`](k8s/base) deploys WSO2 Identity Server 7.3.0 with PostgreSQL 17 and Mailpit. It
  needs cert-manager.
  * `certificates.yaml` creates a private CA and the certificate that WSO2 serves on port 9443.
    The certificate names `localhost` and the in-cluster service.
  * `postgres.yaml` creates the identity, shared, agent identity and test user store databases.
  * `wso2is.yaml` runs two init containers before the server starts. The first one builds the
    TLS keystore from the certificate, adds the CA to the truststore, copies `deployment.toml`
    and downloads the PostgreSQL JDBC driver (with a checksum check). The second one loads the
    database schemas once.
  * `deployment.toml` sets the host name and the proxy port, the PostgreSQL data sources, the
    keystores, the Console and My Account callback URLs, email through Mailpit, tenant deletion
    and the flow `DELETE` rule.
* [`k8s/platform`](k8s/platform) holds the Let's Encrypt `ClusterIssuer`s. Certificates are
  proven with HTTP-01 challenges through Traefik, the ingress controller that k3s includes.
* [`k8s/overlays/lab`](k8s/overlays/lab) adds the public `Ingress`. Traefik presents the
  Let's Encrypt certificate to clients. It then connects to WSO2 over HTTPS and checks WSO2's
  certificate against the private CA (a Traefik `ServersTransport`).

### Use these manifests for production

`k8s/base` is a good starting point, but review these points first:

* Replace the primary keystore (`wso2carbon.p12` with the password `wso2carbon`). The server signs
  tokens with it.
* Use a managed or operator-run PostgreSQL cluster instead of the single `postgres` pod.
* Set resource requests and limits that fit your load, and run more than one replica only with
  WSO2's clustering settings.
* Decide whether tenants and flows may be deleted through the API, and remove those settings from
  `deployment.toml` if not.
* Send mail through a real SMTP server instead of Mailpit.

## Older versions for compatibility checks

`task lab:compat` deploys older WSO2 IS versions next to the lab server. They use the built-in H2
database and the same settings as the Docker server. The default version is 7.2.0.

```shell
task lab:compat VERSIONS="7.2.0"
task lab:compat:forward VERSIONS="7.2.0"   # https://localhost:19720
task testacc BASE_URL=https://localhost:19720 INSECURE=true -- ./...
task lab:compat:down
```

## Ubuntu 25.10 and later

On Ubuntu 25.10 and 26.04, k3s containers can fail to send signals to each other. PostgreSQL then
hangs during recovery, and some programs spin at 100% CPU. The cause is a stricter AppArmor
setting in these releases. The lab avoids the problem by using Ubuntu 24.04 LTS. If you must use a
newer release, give the pods an AppArmor profile that allows signals between containers.
