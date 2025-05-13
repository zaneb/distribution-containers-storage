Containers-Storage Back End for Container Registry
==================================================

`distribution-containers-storage` is a storage plugin library for
<https://github.com/distribution/distribution> that uses
<https://github.com/containers/storage> (i.e. the `containers-storage` option
detailed in `man 5 containers-transports`) as a (read-only) data store. It is
an attempt to mitigate the problem of ever single tool that works with
containers having its own unique on-disk storage format.

Currently there is no support for tags, images must be referenced by digest.

Configuration
-------------

In the registry config file, set:

```
storage:
  containerstorage: {}
```

There are as yet no configuration options for setting the container store.
Currently he registry must run as root in order to avoid permissions errors
with the container store, so in practice this always uses the system's
configured root container store (the one you see with `sudo podman image
list`).

Why This Does Not Work
----------------------

Layer digests are the sha256 sum of the _compressed_ layer diff. This means you
must always have access to the original (compressed) blob file when moving
layers around. This works when moving images between registries, because the
blobs are simply downloaded and re-uploaded. But containers-storage does not
store the original blobs; it extracts them to overlayfs layers. While the
uncompressed blobs can be reconstructed, the compression is different resulting
in the blob having an sha256 sum that does not match its digest.
