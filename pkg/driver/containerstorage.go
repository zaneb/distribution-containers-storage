package driver

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/opencontainers/go-digest"
)

type blobFunc func() (io.ReadCloser, error)

type store interface {
	listRepos() ([]string, error)
	listRepoRevisions(string) ([]string, error)
	listRepoLayers(string) ([]string, error)
	listBlobs() ([]string, error)
	getBlob(sha string) (blobFunc, int64, error)
}

func newContainerStorage() (store, error) {
	opts, err := storage.DefaultStoreOptionsAutoDetectUID()
	if err != nil {
		return nil, err
	}
	// This doesn't seem to help at all
	opts.GraphDriverOptions = append(opts.GraphDriverOptions,
		"overlay.ignore_chown_errors=true")
	store, err := storage.GetStore(opts)
	if err != nil {
		return nil, err
	}
	return &containerStorage{
		store: store,
	}, nil
}

type containerStorage struct {
	store storage.Store
}

func (cs *containerStorage) listRepos() ([]string, error) {
	images, err := cs.store.Images()
	if err != nil {
		return nil, err
	}
	names := map[string]struct{}{}
	for _, i := range images {
		for _, n := range i.Names {
			names[n] = struct{}{}
		}
	}
	repos := make([]string, 0, len(names))
	for n, _ := range names {
		repos = append(repos, n)
	}
	return repos, nil
}

func (cs *containerStorage) listRepoRevisions(repo string) ([]string, error) {
	images, err := cs.store.Images()
	if err != nil {
		return nil, err
	}
	shas := []string{}
	for _, i := range images {
		for _, n := range i.Names {
			if n == repo {
				for _, d := range i.Digests {
					shas = append(shas, d.Encoded())
				}
				continue
			}
		}
	}
	return shas, nil
}

func (cs *containerStorage) listRepoLayers(repo string) ([]string, error) {
	images, err := cs.store.Images()
	if err != nil {
		return nil, err
	}
	shas := []string{}
	for _, i := range images {
		for _, n := range i.Names {
			if n == repo {
				nextLayer := i.TopLayer
				for nextLayer != "" {
					layer, err := cs.store.Layer(nextLayer)
					if err != nil {
						return nil, err
					}
					shas = append(shas, layer.CompressedDigest.Encoded())
					nextLayer = layer.Parent
				}
				continue
			}
		}
	}
	return shas, nil
}

func (cs *containerStorage) listBlobs() ([]string, error) {
	images, err := cs.store.Images()
	if err != nil {
		return nil, err
	}
	layers, err := cs.store.Layers()
	if err != nil {
		return nil, err
	}

	shas := make([]string, 0, len(images)+len(layers))
	for _, i := range images {
		for _, d := range i.Digests {
			if d != "" {
				shas = append(shas, d.Encoded())
			}
		}
	}
	for _, l := range layers {
		if d := l.CompressedDigest; d != "" {
			shas = append(shas, d.Encoded())
		}
	}

	return shas, nil
}

func compressBlob(getBlobReader blobFunc) (blobFunc, int64, error) {
	getDiff := func() (io.ReadCloser, error) {
		dr, err := getBlobReader()

		r, w := io.Pipe()
		// containers/storage uses a custom gzip library. We want to
		// use the stdlib one to make it more likely to match the
		// original blob.
		zw, err := gzip.NewWriterLevel(w, gzip.DefaultCompression)
		if err != nil {
			return nil, err
		}
		go func() {
			defer w.Close()
			io.Copy(zw, dr)
			zw.Close()
		}()
		return r, nil
	}
	r, err := getDiff()
	if err != nil {
		return nil, 0, err
	}
	size, err := io.Copy(io.Discard, r)
	if err != nil {
		return nil, 0, err
	}

	return getDiff, size, nil
}

func (cs *containerStorage) getBlob(sha string) (blobFunc, int64, error) {
	errs := []error{}
	shaDigest := digest.NewDigestFromEncoded(digest.Canonical, sha)
	if layers, err := cs.store.LayersByCompressedDigest(shaDigest); err == nil {
		for _, layer := range layers {
			var diffOptions *storage.DiffOptions

			getBlobReader := func() (io.ReadCloser, error) {
				dr, err := cs.store.Diff("", layer.ID, diffOptions)
				if err != nil {
					return nil, fmt.Errorf("could not get diff for blob %s (layer %s): %w", sha, layer.ID, err)
				}
				return dr, nil
			}

			hash := sha256.New()
			r, err := getBlobReader()
			if err != nil {
				return nil, 0, err
			}
			if _, err := io.Copy(hash, r); err != nil {
				return nil, 0, err
			}
			if digest.NewDigest(digest.Canonical, hash) == shaDigest {
				// Layer was created with the same compression library as used
				// by containers/storage (e.g. by buildah), so we can just
				// return it.
				return getBlobReader, layer.CompressedSize, nil
			}

			// Digest doesn't match with default compression, so try compressing
			// the blob using the stdlib gzip (as used by e.g. moby/moby).
			compression := archive.Uncompressed
			diffOptions = &storage.DiffOptions{
				Compression: &compression,
			}
			return compressBlob(getBlobReader)
		}
	} else {
		errs = append(errs, err)
	}

	if images, err := cs.store.ImagesByDigest(shaDigest); err == nil {
		for _, image := range images {
			b, err := cs.store.ImageBigData(image.ID, storage.ImageDigestBigDataKey)
			if err == nil {
				return func() (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader(b)), nil
				}, int64(len(b)), nil
			}
			errs = append(errs, fmt.Errorf("could not get manifest data for blob %s: %w", sha, err))
		}
	} else {
		errs = append(errs, err)
	}
	if image, err := cs.store.Image(shaDigest.Encoded()); err == nil {
		b, err := cs.store.ImageBigData(image.ID, shaDigest.String())
		if err == nil {
			return func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(b)), nil
			}, int64(len(b)), nil
		}
		errs = append(errs, fmt.Errorf("could not get manifest data for blob %s: %w", sha, err))
	} else {
		errs = append(errs, err)
	}

	return nil, 0, fmt.Errorf("blob %s not found: %w", sha, errors.Join(errs...))
}
