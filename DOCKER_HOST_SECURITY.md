# Docker Host Security (DooD hardening)

This devcontainer can talk to the host's Docker daemon (Docker-outside-Docker).
That access is dangerous by default: anything inside the container that can
reach `/var/run/docker.sock` can become root on the host. The CLI ships several
layers you can opt into.

## Layers

1. **Socket mode (`dockerSocket` option on the devcontainer service)**
   - `none` (default): no socket access at all.
   - `proxy`: talk to the daemon through `docker-socket-proxy` (Tecnativa).
     `DOCKER_HOST=tcp://docker-socket-proxy:2375` is injected; the raw socket
     is NOT mounted into the devcontainer.
   - `ro`: socket mounted read-only (limits some attacks, not all).
   - `rw`: legacy direct read-write mount (avoid).
2. **Socket proxy service** — enable the `docker-socket-proxy` compose service
   and pair it with `dockerSocket: proxy`. Toggle which endpoint groups are
   exposed (CONTAINERS/IMAGES/POST/EXEC/VOLUMES) per project.
3. **Host daemon hardening** — `cli/assets/daemon.json` is a recommended
   `/etc/docker/daemon.json` template. Not applied automatically.

## Recommended setup

`docker-socket-proxy` + `dockerSocket: proxy` + host `daemon.json` with
`userns-remap`. Pick this if the workload only needs to manage containers it
itself created and never needs `docker exec` from the proxied side.

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
