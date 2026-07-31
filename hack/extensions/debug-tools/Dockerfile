# Talos system extension providing console debug tooling: busybox + iproute2.
#
# Everything lands under /usr/local, the only writable-by-extensions prefix that
# is on the dashboard's PATH. iproute2 is dynamically linked, so its libraries
# ship alongside it and the binaries are repointed at /usr/local/lib.
#
# Build:
#   docker build -t <ref> hack/extensions/debug-tools
#
# Use:
#   imager ... --system-extension-image <ref>

FROM alpine:3.22 AS builder

ARG TARGETARCH

RUN apk add --no-cache busybox-static iproute2 iproute2-ss iproute2-tc patchelf

WORKDIR /stage

# musl names the loader per-arch; resolve it once and reuse.
RUN <<'EOF'
    set -eu
    mkdir -p rootfs/usr/local/bin rootfs/usr/local/lib

    cp /bin/busybox.static rootfs/usr/local/bin/busybox
    chmod 0755 rootfs/usr/local/bin/busybox

    # Applet symlinks, so `ls`, `cat`, `ps` ... resolve via PATH even if this
    # busybox was not built with a standalone shell.
    for applet in $(rootfs/usr/local/bin/busybox --list); do
        [ "$applet" = "busybox" ] && continue
        ln -sf busybox "rootfs/usr/local/bin/$applet"
    done
EOF

# Real iproute2 replaces the busybox applets of the same name.
RUN <<'EOF'
    set -eu
    for binary in /sbin/ip /sbin/ss /sbin/bridge /sbin/tc /sbin/nstat /sbin/rtmon; do
        [ -f "$binary" ] || continue
        cp "$binary" "rootfs/usr/local/bin/$(basename "$binary")"
    done

    # Collect shared libraries of everything dynamic we just copied.
    for binary in rootfs/usr/local/bin/*; do
        [ -L "$binary" ] && continue
        ldd "$binary" 2>/dev/null | awk '/=>/ {print $3} /ld-musl/ {print $1}' | while read -r lib; do
            [ -f "$lib" ] || continue
            cp -n "$lib" rootfs/usr/local/lib/ || true
        done
    done

    # Repoint the dynamic binaries at the libraries we ship, so they do not
    # depend on anything in the Talos rootfs.
    loader=$(find rootfs/usr/local/lib -name 'ld-musl-*.so.1' -print -quit)
    for binary in rootfs/usr/local/bin/*; do
        [ -L "$binary" ] && continue
        patchelf --print-interpreter "$binary" >/dev/null 2>&1 || continue
        patchelf --set-interpreter "/usr/local/lib/$(basename "$loader")" \
                 --set-rpath /usr/local/lib "$binary"
    done

    chmod 0755 rootfs/usr/local/lib/* rootfs/usr/local/bin/busybox
EOF

# Fail the build rather than ship a broken extension.
RUN <<'EOF'
    set -eu
    test -x rootfs/usr/local/bin/busybox
    test -x rootfs/usr/local/bin/ip
    rootfs/usr/local/bin/busybox sh -c 'echo busybox ok'
    echo "shipping $(ls rootfs/usr/local/bin | wc -l) entries in /usr/local/bin"
EOF

FROM scratch

COPY --from=builder /stage/rootfs /rootfs
COPY manifest.yaml /
