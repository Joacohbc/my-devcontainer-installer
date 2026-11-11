# Reach services across the boundary

A tunnel works in both directions: from here into the container (the common
case) and from the container back to something running on this machine.

## From here into the container

**Prefer `agent forward` or `access route add` over publishing ports with `--ports`.**
A published port is a permanent, host-wide change to the project (it changes
the compose file and stays open on every future `workspace up`), while a tunnel or a
route is created on demand and torn down when you are done — it needs no
regenerate/rebuild and cannot collide with another project's port on the same
host. Reach for `--ports` only when the user explicitly wants the port
published (e.g. so something outside your control can dial it directly).

    devcontainer-cli agent forward 3000              # 127.0.0.1:3000 -> container:3000
    devcontainer-cli agent forward 8080:80           # 127.0.0.1:8080 -> container:80
    devcontainer-cli agent forward 5432:postgres:5432  # reach a sibling service
    devcontainer-cli agent forward 0.0.0.0:8000:8000   # listen on every interface here

A leading IP of this machine (`0.0.0.0`, a LAN or Tailscale address) is the
**bind address**, like `ssh -L`: the tunnel listens there instead of on
`127.0.0.1`, so other machines can reach it. Only do that when the user asks
for it — anything that can reach the address reaches the container port. A
`reverse:` tunnel takes no bind address (its SSH server keeps it on the
container's loopback), and the CLI rejects the combination.

`agent forward` tunnels over SSH and stays in the **foreground** until
interrupted, so run it in the background (or in a separate terminal) if you need
to keep working.

For a stable local domain instead of a raw port (e.g. to hand a URL to
something else, or keep it running across sessions) use `access route add` — see
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
  SSH session, which runs as `devuser`, so `reverse:80:80` is refused;
  `reverse:80:8080` reaches the host's port 80 on the container's 8080.
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

For a real SSH session rather than a `docker exec`:

    devcontainer-cli agent ssh -- go version
    devcontainer-cli access ssh --via me@docker-host -c mycontainer -- uname -a

Sibling services are reached **inside** the container by compose service name
(`postgres`, `redis`, `mongo`), never `localhost`.

To let an unrelated container talk to the project:

    devcontainer-cli access network connect other-app

To route a local domain to a container service via the global Caddy reverse proxy:

    devcontainer-cli access route add 3000                  # http://<workspace>-3000.devcli.localhost
    devcontainer-cli access route add 8000 --domain api     # http://api.devcli.localhost
    devcontainer-cli access route ls                        # list active routes
    devcontainer-cli access route status                    # check global Caddy router status
