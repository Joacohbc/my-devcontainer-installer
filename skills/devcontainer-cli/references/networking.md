# Reach services across the boundary

Every project runs on a private `/28` bridge network (`<workspace>-network`) with **zero host ports published by default** (including internal SSH on port `22`). A tunnel works in both directions: from `127.0.0.1` here into the container (the common case) and from the container's loopback back to something running on this machine.

## Command execution priority: `agent exec` vs `agent ssh`

- **Container is local -> prioritize `agent exec -w -- <cmd>`**: uses `docker exec`, needs no `sshd`, runs directly as `devuser`, and has zero SSH overhead. Use `agent ssh -- <cmd>` locally only when you genuinely need SSH features (`ssh-agent` forwarding, SSH PTY semantics, `scp`, or a tool calling `ssh`).
- **Container is on a remote Docker host (`--via`) -> prioritize `agent ssh --via <user@host> -c <container> -- <cmd>`**: runs ephemerally as `devuser` without touching `~/.ssh/` (`access shell --via ... --user devuser` is the fallback when the target container has no `sshd`). If the user wants another machine set up with the managed key, point them to `devcontainer-cli config ssh-key --export <user@host>`.

Note: `agent ssh` and `agent forward` run `--ephemeral` by default — they automatically inject the shared managed public key (`id_devcontainer`) into the container and dial port `22` without reading or mutating `~/.ssh/config` or `known_hosts`.

## From here into the container

**Prefer `agent forward` or `access route add` over publishing ports with `--ports`.**
A published port is a permanent, host-wide change to the project (it modifies
the compose file, stays open on every future `workspace up`, and binds to `127.0.0.1`
unless an explicit IP is given), while a tunnel or a route is created on demand and
torn down when you are done — it needs no regenerate/rebuild and cannot collide with
another project's port on the same host. Reach for `--ports` only when the user
explicitly wants the port published.

    devcontainer-cli agent forward 3000                    # 127.0.0.1:3000 -> container:3000
    devcontainer-cli agent forward 8080:80                 # 127.0.0.1:8080 -> container:80
    devcontainer-cli agent forward 5432:postgres:5432      # reach a sibling service
    devcontainer-cli agent forward 5432 --service postgres # same via --service flag
    devcontainer-cli agent forward 100.x.y.z:8000:8000     # bind to a VPN/Tailscale IP here

By default `agent forward` binds explicitly to **`127.0.0.1`** (both for loopback isolation and browser *Secure Context* compatibility). A leading IP of this machine (for example a Tailscale `100.x.y.z` address or `0.0.0.0`) changes the **bind address** like `ssh -L`: only expose a non-loopback IP (and avoid `0.0.0.0` unless explicitly asked) when the user requests remote access — anything that can reach that address reaches the container port. A `reverse:` tunnel takes no bind address and rejects `--service` (its SSH server keeps it on the container's loopback).

`agent forward` tunnels over SSH and stays in the **foreground** until
interrupted, so run it in the background (or in a separate terminal) if you need
to keep working.

For a stable local domain instead of a raw port, use `access route add` — see
below — rather than publishing.

Only publish a port permanently (via `--ports`) when a tunnel/route genuinely
does not fit the case:

    devcontainer-cli agent create --ports 3000:3000

## From the container back to this machine

The host's `localhost` is **not** the container's. A database, an API or a dev
server running on this machine is unreachable from inside until you open a
reverse tunnel for it — the same command with a `reverse:` prefix (or
`--reverse`). The container then finds it on its own `127.0.0.1`.

    devcontainer-cli agent forward reverse:5432       # container:5432 -> host:5432
    devcontainer-cli agent forward reverse:5432:15432 # container:15432 -> host:5432
    devcontainer-cli agent forward reverse:5432:192.168.1.20:5432  # a LAN host

The port fields keep their meaning in both directions: the first is always this
machine's port, the last always the container's — only the arrow flips.

Three things to know before reaching for it:

- **The container port must be 1024 or above.** The listener is bound by the
  SSH session on container loopback (`127.0.0.1`), which runs as `devuser`, so
  `reverse:80:80` is refused; `reverse:80:8080` reaches the host's port 80 on
  the container's 8080.
- **The host service must answer on an address this machine can dial.** A
  server bound to `127.0.0.1` here is fine (the tunnel dials from here); one
  that is not running yet is not.
- **A sibling compose service is not this.** `postgres`, `redis` and `mongo`
  from the project's own stack are reached inside the container by service
  name — no tunnel at all. Use a reverse tunnel only for something running
  outside the stack.

It is still a foreground process, and still torn down when you stop it. To make
one permanent for a project, put it in its `forward_ports` instead — a
`reverse:` spec is accepted there too:

    devcontainer-cli agent create --forward-ports reverse:5432,3000

## Other stacks and HTTP local routing (`network` & `route`)

Sibling services are reached **inside** the container by compose service name
(`postgres`, `redis`, `mongo`), never `localhost`.

To attach an unrelated running Docker container to `<workspace>-network` on the
fly with a predictable DNS alias (instead of publishing ports on the host):

    devcontainer-cli access network connect other-app --alias billing-api
    devcontainer-cli access network disconnect other-app

To route a local `*.devcli.localhost` domain (resolved to `127.0.0.1` without
editing `/etc/hosts`) through the shared Caddy proxy (`devcli-router`, bound to
`127.0.0.1:80/443`). Like `agent forward`, `route add` stays in the
**foreground** and unregisters the route on `Ctrl+C`, so run it in the
background if you need the terminal back:

    devcontainer-cli access route add 3000                  # http://<workspace>-3000.devcli.localhost
    devcontainer-cli access route add 8000 --domain api     # http://api.devcli.localhost
    devcontainer-cli access route ls                        # list active routes
    devcontainer-cli access route rm api.devcli.localhost   # remove a route manually
    devcontainer-cli access route status                    # check global Caddy router status
    devcontainer-cli access route stop                      # stop global devcli-router

## Remote private networks (Tailscale and Cloudflare Tunnel)

Read the matching reference before setting up remote access beyond the local machine:

- **Tailscale (host-side default, or standalone Docker gateway + `access network connect`)**: `references/tailscale.md`.
- **Cloudflare Tunnel (`cloudflared` module inside container + WARP)**: `references/cloudflare-tunnel.md`.
