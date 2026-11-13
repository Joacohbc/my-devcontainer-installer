# Tailscale (Host-Side or Docker Gateway + `access network connect`)

Connect to devcontainers across a tailnet using either **Option A: Tailscale on the Docker host (default)** or **Option B: a single standalone `tailscale/tailscale` Docker gateway attached via `devcontainer-cli access network connect`**.

- Official documentation:
  - Install Tailscale on Linux / host: <https://tailscale.com/docs/install/linux> (<https://tailscale.com/kb/1031/install-linux>)
  - HTTPS inside tailnet (`tailscale serve`): <https://tailscale.com/kb/1312/serve>
  - Official Docker containers reference: <https://tailscale.com/docs/features/containers/docker>

## Architectural rule

- Keep Tailscale **outside** individual devcontainer stacks (`Dockerfile` and `.dc_<workspace>/build/docker-compose.yml` stay untouched so `mode=custom`, `profiles`, `image`, and `agent run` work identically).
- Do **not** install `tailscaled` inside devcontainers, and never run raw `docker network` or edit `.dc_<workspace>/` files.
- **Never bind tunnels to `0.0.0.0` by default.** Bind to `127.0.0.1` (default) or explicitly to the host's Tailscale IP (`100.x.y.z`).

---

## Option A: Tailscale on the Docker host (recommended default)

### 1. Remote SSH (`--via`)

From a client machine on the same tailnet, configure the managed SSH alias through the Docker host's Tailscale address or MagicDNS name:

```bash
devcontainer-cli access ssh --via <user>@<tailscale-host> --container <workspace>-devcontainer-ssh
ssh <workspace>-devcontainer-ssh
```

The private key stays on the client and container port `22` stays unexposed.

### 2. Expose container web ports without breaking Secure Contexts

Browsers disable `crypto.randomUUID()`, WebCrypto, and Service Workers on plain HTTP to non-loopback IPs (`http://100.x.y.z:<port>`). Pick the pattern matching the caller:

1. **Desktop browser (zero app changes, `localhost` Secure Context)**:
   - On a client already configured with `--via`:
     ```bash
     devcontainer-cli access forward 3000 --alias <workspace>-devcontainer-ssh
     ```
   - Or forward to loopback on the server and tunnel from the client:
     ```bash
     # Server host:
     devcontainer-cli agent forward 3000
     # Client machine:
     ssh -L 3000:127.0.0.1:3000 <user>@<tailscale-host>
     ```
   Open `http://localhost:3000` on the client.

2. **Mobile / PWA / multi-device (automatic TLS via `tailscale serve`)**:
   Requires MagicDNS and HTTPS Certificates enabled in Tailscale Admin Console.
   ```bash
   # Keep the SSH forward private on loopback:
   devcontainer-cli agent forward 3000
   # Serve over HTTPS inside the tailnet (run in another host session):
   sudo tailscale serve --bg 3000
   ```
   Access `https://<hostname>.<tailnet>.ts.net`. Check or tear down with `tailscale serve status` / `tailscale serve reset`.
   *(For Vite 6+ dev servers, add the `*.ts.net` host to `server.allowedHosts` in `vite.config.ts`.)*

3. **Headless APIs / non-browser TCP services**:
   Discover the host's Tailscale IP (`tailscale ip -4`) and bind the tunnel strictly to that private interface:
   ```bash
   devcontainer-cli agent forward 100.x.y.z:8000:8000
   ```
   For multi-port project workflows, declare `--forward-ports` at creation (`devcontainer-cli agent create --forward-ports 3000,8000`) and open them together on the host with `devcontainer-cli access forward`.

### 3. Reach a tailnet service from inside the container (`reverse:`)

Expose a database or service on another tailnet node to the container's loopback (container port must be `>= 1024`):

```bash
devcontainer-cli agent forward reverse:5432:<tailnet-ip-or-magicdns>:5432
```

---

## Option B: Standalone Tailscale Docker gateway + `access network connect`

Use when Tailscale cannot be installed on the host OS and `/dev/net/tun` + `NET_ADMIN` + `ip_forward=1` are available for a single shared infrastructure container running outside any project directory (`tailscale-gateway` subnet router):

1. Deploy the standalone `tailscale/tailscale` subnet router outside workspace folders (`TS_USERSPACE=false`, `/dev/net/tun`, `NET_ADMIN`, `NET_RAW`, `net.ipv4.ip_forward=1`, `TS_ROUTES=172.25.0.0/16` or specific project `DOCKER_SUBNET` CIDRs, persistent `TS_STATE_DIR` volume, `TS_AUTH_ONCE=true`), then approve the advertised route in Tailscale Admin Console (or ACL `autoApprovers`).
2. Attach it to any running workspace's private Docker bridge network using the CLI (never raw `docker network`):
   ```bash
   devcontainer-cli access network connect tailscale-gateway
   ```
3. Read `DEVCONTAINER_IP` from `.dc_<workspace>/build/.env` (for example `172.25.0.14`) and connect over the tailnet:
   ```bash
   ssh devuser@172.25.0.14
   # For Secure Context in browser apps:
   ssh -L 3000:127.0.0.1:3000 devuser@172.25.0.14
   ```
4. Detach the gateway before bringing the project down or destroying it so Compose can remove the workspace network cleanly:
   ```bash
   devcontainer-cli access network disconnect tailscale-gateway
   ```

Constraints vs Option A:
- `tailscale serve` cannot proxy directly to another container name or IP (`only localhost or 127.0.0.1 proxies are currently supported`), so HTTPS serve is not direct without a co-located reverse proxy.
- Two hosts advertising overlapping `172.25.0.0/16` ranges in the same tailnet collide; advertise per-project `DOCKER_SUBNET` CIDRs if multiple Docker hosts share a tailnet.
