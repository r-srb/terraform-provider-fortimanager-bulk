# FortiManager Terraform Provider (Bulk Read Cache Fork)

This is a fork of [fortinetdev/terraform-provider-fortimanager](https://github.com/fortinetdev/terraform-provider-fortimanager) with a **bulk read cache** that replaces ~35,000 individual FortiManager API GETs with ~78 paginated bulk GETs during `terraform plan`, reducing refresh time from 30+ minutes to under 2 minutes for large deployments.

**Cached resource types:**
- `fortimanager_object_firewall_address` / `address6`
- `fortimanager_object_firewall_addrgrp` / `addrgrp6`
- `fortimanager_object_firewall_service_custom` / `service_group`
- `fortimanager_packages_pblock_firewall_policy`

Create / Update / Delete operations are untouched — only Read (refresh) is accelerated.

---

- Website: https://www.terraform.io
- [![Gitter chat](https://badges.gitter.im/hashicorp-terraform/Lobby.png)](https://gitter.im/hashicorp-terraform/Lobby)
- Mailing list: [Google Groups](http://groups.google.com/group/terraform-tool)

<img src="https://www.datocms-assets.com/2885/1629941242-logo-terraform-main.svg" width="600px">

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) 1.0+
- [Go](https://golang.org/doc/install) 1.21+ (to build the provider plugin)
- The provider can cover FortiManager 6.4 to 7.4 versions.

## Building the Provider

### On macOS (for local development)

```sh
git clone <this-repo> terraform-provider-fortimanager-bulk
cd terraform-provider-fortimanager-bulk
go build -o terraform-provider-fortimanager .
```

### Cross-compile for Linux amd64 (Ubuntu 24.04)

Run this on your Mac (or any machine with Go installed):

```sh
cd terraform-provider-fortimanager-bulk
GOOS=linux GOARCH=amd64 go build -o terraform-provider-fortimanager_linux_amd64 .
```

The resulting binary is ~75 MB and has no external dependencies (statically linked).

---

## Installing on Ubuntu 24.04

Choose one of the two methods below. **Method A** is simpler for a single server.
**Method B** is better for CI/CD pipelines or shared servers where you want
`terraform init` to work normally.

### Method A — dev_overrides (simple, no `terraform init` needed)

1. **Copy the binary to the server:**

   ```sh
   # On your Mac
   scp terraform-provider-fortimanager_linux_amd64 user@server:~/fortimanager-provider/terraform-provider-fortimanager
   ssh user@server "chmod +x ~/fortimanager-provider/terraform-provider-fortimanager"
   ```

2. **Configure `~/.terraformrc` on the server:**

   ```sh
   cat > ~/.terraformrc <<'EOF'
   provider_installation {
     dev_overrides {
       "fortinetdev/fortimanager" = "/home/user/fortimanager-provider"
     }
     direct {}
   }
   EOF
   ```

   Replace `/home/user/fortimanager-provider` with the actual directory containing the binary.

3. **Run Terraform** — skip `terraform init` for the fortimanager provider:

   ```sh
   cd /your/terraform/project
   terraform plan   # dev_overrides bypasses the registry for this provider
   ```

   You will see this warning on every run — it is expected:

   ```
   Warning: Provider development overrides are in effect
     - fortinetdev/fortimanager in /home/user/fortimanager-provider
   ```

---

### Method B — filesystem_mirror (production-grade, works with `terraform init`)

This places the binary in Terraform's standard plugin directory structure so
`terraform init` installs it locally without contacting the registry.

1. **Copy the binary to the server into the mirror directory:**

   ```sh
   # On your Mac — copy the binary
   scp terraform-provider-fortimanager_linux_amd64 user@server:/tmp/

   # On the server — place it in the mirror directory
   VERSION=1.17.0
   PLUGIN_DIR=~/.terraform.d/plugins/registry.terraform.io/fortinetdev/fortimanager/${VERSION}/linux_amd64
   mkdir -p "${PLUGIN_DIR}"
   cp /tmp/terraform-provider-fortimanager_linux_amd64 "${PLUGIN_DIR}/terraform-provider-fortimanager_v${VERSION}"
   chmod +x "${PLUGIN_DIR}/terraform-provider-fortimanager_v${VERSION}"
   ```

2. **Configure `~/.terraformrc` on the server:**

   ```sh
   cat > ~/.terraformrc <<'EOF'
   provider_installation {
     filesystem_mirror {
       path    = "/home/user/.terraform.d/plugins"
       include = ["fortinetdev/fortimanager"]
     }
     direct {
       exclude = ["fortinetdev/fortimanager"]
     }
   }
   EOF
   ```

   Replace `/home/user` with your actual home directory path.

3. **Run `terraform init` then `terraform plan`:**

   ```sh
   cd /your/terraform/project
   terraform init    # installs from the local mirror, no registry contact
   terraform plan
   ```

   No warning is shown. The provider behaves exactly like a registry-installed one.

---

## Verifying the bulk cache is active

Set `TF_LOG=INFO` and check that a small number of paginated bulk GETs appear
instead of thousands of per-object GETs:

```sh
TF_LOG=INFO terraform plan 2>&1 | grep "Request URL" | sort | uniq -c | sort -rn
```

**Before** (upstream provider — one GET per object):
```
1  Request URL: /pm/config/adom/TEST/obj/firewall/address/host-1
1  Request URL: /pm/config/adom/TEST/obj/firewall/address/host-2
...  (repeats thousands of times)
```

**After** (this fork — paginated bulk GETs, 500 objects per page):
```
20  Request URL: /pm/config/adom/TEST/obj/firewall/address    ← 10,000 addresses in 20 pages
 4  Request URL: /pm/config/adom/TEST/obj/firewall/addrgrp
 2  Request URL: /pm/config/adom/TEST/obj/firewall/service/custom
```

Time the plan to confirm the speedup:

```sh
time terraform plan > /dev/null
```

---

## Running the tests

```sh
# Cache logic (pure Go, no network)
go test -vet=off -v ./fmg/... -run "TestGetOrLoad|TestInvalidate|TestGetOrLoadPolicyIndex"

# Pagination logic (uses an in-process HTTPS test server)
go test -vet=off -v ./sdk/sdkcore/... -run "TestBulkRead"
```

> **Note:** The upstream codebase has ~2,000 pre-existing `go vet` warnings
> (non-constant format strings in `log.Printf`). The `-vet=off` flag is
> required to run tests; it does not suppress real compilation errors.

---

## Developing the Provider

If you wish to work on the provider, you'll first need Go installed on your
machine (version 1.21+ is required).

To compile the provider locally:

```sh
go build -o terraform-provider-fortimanager .
```

Configure `~/.terraformrc` with `dev_overrides` pointing to this directory
(see Method A above) and run `terraform plan` directly — no `terraform init`
needed.

## Upstream

This fork is based on
[fortinetdev/terraform-provider-fortimanager](https://github.com/fortinetdev/terraform-provider-fortimanager)
v1.17.0. Only the Read path for the resource types listed above is modified.
