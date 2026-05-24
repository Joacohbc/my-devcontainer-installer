# Docker access & sandboxing

A devcontainer that needs Docker has two fundamentally different models:

- **DinD (Docker-in-Docker)** — run an *isolated* daemon in a sidecar. Workloads
  you launch never touch the host. This is the only model that is a real
  sandbox. Use it when you just need *a* Docker engine for your own project.
- **DooD (Docker-outside-of-Docker)** — talk to the *host* daemon over its
  socket. Convenient (shares the host image cache, can manage host containers)
  but it is **not a sandbox**: anything that can reach `/var/run/docker.sock` —
  proxied or not — can become root on the host. Hardening only narrows the path.

## Socket mode (`dockerSocket` option on the devcontainer service)

- `none` (default): no Docker access at all.
- `dind`: isolated rootless Docker-in-Docker engine (recommended sandbox).
  Requires the `docker-dind` compose service. `DOCKER_HOST=tcp://docker-dind:2375`
  is injected; no host socket is mounted. The engine sits on a dedicated
  `*-engine-network` joined only by the devcontainer and the engine.
- `proxy`: host daemon via `docker-socket-proxy` (Tecnativa). Hardened DooD, not
  a sandbox. Requires the `docker-socket-proxy` service.
- `rw`: direct read-write host socket mount. Equivalent to handing out host
  root. Avoid.

> A read-only (`:ro`) socket mount was intentionally **removed**: `:ro` on a unix
> socket does not restrict the Docker API (the mount flag blocks file writes, not
> `connect()`), so it gave a false sense of safety equal to `rw`.

If you select `proxy` or `dind` but the backing service is not enabled, the
generator falls back to `none` (no access) — never to a silent socket mount.

## Recommended setup — sandbox (DinD)

Enable the `docker-dind` service + `dockerSocket: dind`. The engine runs
rootless, so a workload-container escape lands in an unprivileged user namespace
rather than as host root. Caveat: the dind container itself is `privileged`
(needs cgroups/overlayfs/netns), so compromising *that* container is host root —
but your code runs in the devcontainer, which only reaches it over TCP.

## Recommended setup — host daemon (DooD)

`docker-socket-proxy` + `dockerSocket: proxy` + host `daemon.json` with
`userns-remap`. **Set `POST=0`** on the proxy unless you truly need to create
containers: with `POST=1` an attacker can create a privileged container that
mounts host `/`, i.e. proxy access ≈ host root. Pick this only if the workload
needs the host's images/containers, not isolation.

## Installing the host daemon.json

```bash
sudo cp cli/assets/daemon.json /etc/docker/daemon.json
sudo systemctl restart docker
```

`userns-remap: default` remaps container UID 0 to a non-root host UID. After
turning it on Docker creates a fresh storage tree under
`/var/lib/docker/<uid>.<gid>/` — existing images/volumes are NOT migrated, so
either back them up first or accept the reset. Pin a specific UID range in
`/etc/subuid` / `/etc/subgid` if you want stable host UIDs across hosts.

## What this does NOT cover

- Rootless Docker on the host. If you can, run the daemon rootless instead of
  rootful + userns-remap. Caveat: bind-mounting host paths into containers
  becomes more constrained.
- Seccomp / AppArmor profiles per container. The proxy service ships with
  `no-new-privileges`, `cap_drop: ALL`, and a minimal cap set; the devcontainer
  itself does not.
- Auditing. Consider `auditd` rules on `/var/run/docker.sock` if you must mount
  it directly.
