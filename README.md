Containers-Storage Back End for Container Registry
==================================================

`distribution-containers-storage` is a storage plugin library for
[`distribution/distribution`](https://github.com/distribution/distribution)
that uses [`containers-storage`](https://github.com/containers/storage) (i.e.
the `containers-storage` option detailed in `man 5 containers-transports`) as a
(read-only) data store. It is an attempt to slightly mitigate the problem of
every single tool that works with containers having its own unique on-disk
storage format.

Currently there is no implementation of tags; images must be referenced by
digest.

Use
---

Import this library into the distribution registry. Currently the library is
implemented against [`openshift/docker-distribution` at commit
`ac5742e896d4`](https://github.com/openshift/docker-distribution/tree/ac5742e896d480763c85f9b65e3c331aa0613552).
For local development, add a replace directive pointing at your local path,
like:

```
replace github.com/zaneb/distribution-containers-storage => ../distribution-containers-storage
```

Compile the registry with `-tags "exclude_graphdriver_btrfs exclude_graphdriver_aufs exclude_graphdriver_devicemapper exclude_graphdriverr_zfs"`.

Configuration
-------------

In the registry config file, set:

```
storage:
  containerstorage: {}
```

There are as yet no configuration options for setting the container store.
Currently the registry must run as root in order to avoid permissions errors
with the container store, so in practice this always uses the system's
configured root container store (the one you see with `sudo podman image
list`).

When This Does Not Work
-----------------------

Layer digests are the sha256 sum of the _compressed_ layer diff. This means you
must always have access to the original (compressed) blob file when moving
layers around. This always works when moving images between registries, because
the blobs are simply downloaded and re-uploaded. But containers-storage does
not store the original blobs; it extracts them to overlayfs layers. While the
uncompressed blobs can be reconstructed, it applies its own gzip compression.

We try both the built-in gzip library [used by
containers-storage](https://github.com/containers/storage/commit/6ef3b9dafaf15a789aa39ac63edfaad0278a57a6)
and the golang stdlib gzip library. **This works only for layers that were
originally built using a toolchain that uses one of these libraries with the
same settings**. This includes moby and recent versions of buildah.
Theoretically there could exist layers built by other tools where the
compression is different, resulting in the blob having an sha256 sum that does
not match its digest (and likely a different size as well, resulting in HTTP
errors). The existing tools could also change their compression algorithms at
some point in the future, as some of them have in the past. In fact, many tools are in the process of switching to zstd:chunked compression by default.
