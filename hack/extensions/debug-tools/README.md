# Talos console debug shell

Stock Talos ships no shell at all, so a node that loses its network is
undebuggable: `talosctl` cannot reach it and the console offers only the
dashboard. This branch adds an interactive shell reachable from the dashboard.

## What it does

Press **`F9`** or **`Ctrl+]`** on the console: the dashboard UI suspends, the
terminal is handed to an interactive shell, and the dashboard is restored when
the shell exits (`exit` or `Ctrl+D`).

`F9` exists because remote consoles (IPMI/SOL, VNC) routinely fail to deliver
`Ctrl+]` — their keymaps do not necessarily produce `0x1D`. Function keys come
through reliably.

The footer advertises the binding when a shell is available. No hint means the
image ships no shell and the binding is inert.

## Why it is split in two parts

The binaries are delivered as a **system extension** (`debug-tools`: busybox +
iproute2 into `/usr/local`). The key binding cannot be: extensions only deliver
files and services, and the extension service spec has no tty, so the dashboard
itself had to be patched.

The shell is enabled automatically when the image ships one, so the extension is
the opt-in. Disable explicitly with `talos.dashboard.shell=0`.

## Security

When the shell is available the dashboard keeps root and its capabilities — the
shell inherits them, and an unprivileged shell cannot do what it is needed for
(`ip link set` and friends). That means **physical or IPMI console access grants
root**. Do not install the extension on production nodes.

## Building

The Talos boot assets (kernel, initramfs, rootfs) live in the **imager** image,
not in `installer-base`. Building only `installer-base` produces an image with
stock Talos binaries, so the imager must be built from this branch:

```bash
# 1. extension with busybox + iproute2 (OCI layout, no registry needed)
docker buildx build --platform linux/amd64 \
  --output type=oci,dest=/out/debug-tools,tar=false \
  hack/extensions/debug-tools

# 2. imager built from this branch - this is what carries the patched binaries
make target-imager PLATFORM=linux/amd64 TAG=debug-shell \
  TARGET_ARGS="--output type=docker,dest=/out/imager.tar"
docker load -i /out/imager.tar

# 3. the image itself
docker run --rm -i --privileged -v /dev:/dev -v /out:/out -w /out \
  <loaded-imager-image> - <<'EOF'
arch: amd64
platform: metal
version: v1.12.1
customization:
  extraKernelArgs:
    - console=ttyS0,115200n8
    - console=tty0
    - panic=0
input:
  systemExtensions:
    - ociPath: /out/debug-tools
output:
  kind: image
  outFormat: raw
EOF
```

Two notes for air-gapped builds:

- `ociPath` / `tarballPath` avoid a registry entirely.
- leave `input.baseInstaller` unset and imager silently falls back to pulling
  `ghcr.io/siderolabs/installer` — pin it explicitly when offline.

`imager` needs loop partition device nodes. In a container whose `/dev` is a
plain tmpfs they never appear and `mkfs` fails; mount a real devtmpfs and pass
that as the imager container's `/dev`.

## Verified

Built from this branch and booted as a KubeVirt VM, checked over VNC:

- dashboard reports version `debug-shell`, logs
  `dashboard: debug shell enabled, running dashboard privileged`
- footer shows the `F9 / Ctrl+]: Shell` hint
- both `F9` and `Ctrl+]` open the shell
- in the shell: `uid=0 gid=0`, `ip utility, iproute2-6.15.0`,
  `ip -o -4 addr show` returns the live interface list
- `exit` restores the dashboard
