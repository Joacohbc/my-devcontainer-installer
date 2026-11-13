# Cloudflare Tunnel (Private Network and WARP)

Expose the devcontainer and its compose network privately via Cloudflare Zero Trust and WARP without installing a tunnel daemon on the host or opening router/firewall ports.

- Official documentation: <https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/private-net/>

## 1. Provision the `cloudflared` module

`cloudflared` is a **Dockerfile module** under `mode=custom` (not included in prebuilt `mode=profiles` images). Add it alongside the project's existing modules:

```bash
devcontainer-cli agent create --modules <modules>,cloudflared
```

Read `.dc_<workspace>/build/.env` (safe to read on the host) to get the project's generated subnet and container IP:

```bash
DOCKER_SUBNET=172.25.0.0/28
DEVCONTAINER_IP=172.25.0.14
```

## 2. Run the tunnel inside the container

Never bake tunnel tokens into `Dockerfile` layers or commit them. Pass the connector token at runtime inside the container:

1. **Named tunnel with connector token (recommended for Private Network + WARP)**:
   ```bash
   devcontainer-cli agent exec -- cloudflared tunnel run --token <TOKEN>
   ```
   To leave a non-TUI connector running in the background after the command exits, detach it with `nohup` + `disown`:
   ```bash
   devcontainer-cli agent exec -- bash -lc 'nohup cloudflared tunnel run --token "$TUNNEL_TOKEN" > /tmp/cloudflared.log 2>&1 < /dev/null &'
   ```
2. **Ephemeral Quick Tunnel (no account/token; URL changes every run)**:
   ```bash
   devcontainer-cli agent exec -- cloudflared tunnel --url http://localhost:8080
   ```
3. **Named tunnel via login (`cloudflared tunnel login`)**:
   Requires an interactive login (`~/.cloudflared/cert.pem`), which also enables CLI route management:
   ```bash
   devcontainer-cli agent exec -- cloudflared tunnel route ip add 172.25.0.0/28 <tunnel-name-or-uuid>
   ```

> **Security warning**: All tunnel modes expose the target port without built-in authentication unless a Cloudflare Access policy is configured in Zero Trust.

## 3. Wire Zero Trust Private Network and WARP client

1. In Cloudflare Zero Trust Dashboard (**Networks** -> **Tunnels** -> **Private Network**), add the exact `DOCKER_SUBNET` CIDR from `.dc_<workspace>/build/.env` (e.g. `172.25.0.0/28`) with no *Public Hostname* if strictly private.
2. In **WARP Client** device profile **Split Tunnels**, ensure that CIDR is **Included** (Include mode) or **Not Excluded** (Exclude mode).
3. From a host running WARP, connect directly to `DEVCONTAINER_IP`:
   ```bash
   ssh devuser@172.25.0.14
   ```
