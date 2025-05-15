package driver

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

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

func imageRepo(name string) (string, error) {
	uri, err := url.Parse("docker://" + name)
	if err != nil {
		return name, err
	}
	if i := strings.LastIndex(uri.Path, ":"); i >= 0 {
		uri.Path = uri.Path[:i]
	}
	if i := strings.LastIndex(uri.Path, "@"); i >= 0 {
		uri.Path = uri.Path[:i]
	}
	uri.Path = uri.Path[0:1] + strings.ReplaceAll(uri.Path[1:], "/", "_")
	return strings.TrimPrefix(uri.String(), "docker://"), nil
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
			repo, err := imageRepo(n)
			if err != nil {
				return nil, err
			}
			names[repo] = struct{}{}
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
			if iRepo, err := imageRepo(n); err == nil && iRepo == repo {
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

func (cs *containerStorage) getBlob(sha string) (blobFunc, int64, error) {
	errs := []error{}
	shaDigest := digest.NewDigestFromEncoded(digest.Canonical, sha)
	if layers, err := cs.store.LayersByCompressedDigest(shaDigest); err == nil {
		for _, layer := range layers {
			getDiff := func() (io.ReadCloser, error) {
				compression := archive.Uncompressed
				dr, err := cs.store.Diff("", layer.ID, &storage.DiffOptions{
					Compression: &compression,
				})
				if err != nil {
					return nil, fmt.Errorf("could not get diff for blob %s (layer %s): %w", sha, layer.ID, err)
				}
				r, w := io.Pipe()
				// containers/storage uses a custom gzip library. We want to
				// use the stdlib one to make it more likely to match the
				// original blob (though it doesn't seem to help).
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
			// This isn't particularly efficient, but the actual compressed
			// image has a different size from layer.CompressedSize, presumably
			// because the compression algorithm applied by containers/image is
			// different to the one that originally created the blob.
			// Calculating the size directly allows us to avoid HTTP
			// "unexpected EOF" errors and prove that the SHA is also different,
			// which is the bigger problem.
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
	} else {
		errs = append(errs, err)
	}

	if images, err := cs.store.ImagesByDigest(shaDigest); err == nil {
		for _, image := range images {
			b, err := cs.store.ImageBigData(image.ID, storage.ImageDigestBigDataKey)
			if err != nil {
				return nil, 0, fmt.Errorf("could not get manifest data for blob %s: %w", sha, err)
			}
			return func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(b)), nil
			}, int64(len(b)), nil
		}
	} else {
		errs = append(errs, err)
	}

	return nil, 0, fmt.Errorf("blob %s not found: %w", sha, errors.Join(errs...))
}
