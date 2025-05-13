package driver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"
)

type repoList struct {
	filePath
}

func (rl *repoList) Reader() (io.ReadCloser, error) {
	return nil, errors.New("is a directory")
}

func (rl *repoList) Stat() (storagedriver.FileInfo, error) {
	return storagedriver.FileInfoInternal{
		storagedriver.FileInfoFields{
			Path:  rl.path(),
			IsDir: true,
		},
	}, nil
}

func (rl *repoList) List() ([]string, error) {
	repos, err := rl.store.listRepos()
	if err != nil {
		return nil, err
	}
	return rl.children(repos...), nil
}

type repo struct {
	filePath
	repo string
}

func (r *repo) Reader() (io.ReadCloser, error) {
	return nil, errors.New("is a directory")
}

func (r *repo) Stat() (storagedriver.FileInfo, error) {
	repos, err := r.store.listRepos()
	if err != nil {
		return nil, err
	}
	for _, repo := range repos {
		if r.repo == repo {
			return storagedriver.FileInfoInternal{
				storagedriver.FileInfoFields{
					Path:  r.path(),
					IsDir: true,
				},
			}, nil
		}
	}
	return nil, storagedriver.PathNotFoundError{Path: r.path()}
}

func (r *repo) List() ([]string, error) {
	if _, err := r.Stat(); err != nil {
		return nil, err
	}
	return r.children("_layers", "_manifests"), nil
}

type layerList struct {
	filePath
	repo string
}

func (ll *layerList) layers() ([]string, error) {
	return ll.store.listRepoLayers(ll.repo)
}

func (ll *layerList) Reader() (io.ReadCloser, error) {
	return nil, errors.New("is a directory")
}

func (ll *layerList) Stat() (storagedriver.FileInfo, error) {
	if _, err := ll.layers(); err != nil {
		return nil, err
	}
	return storagedriver.FileInfoInternal{
		storagedriver.FileInfoFields{
			Path:  ll.path(),
			IsDir: true,
		},
	}, nil
}

func (ll *layerList) List() ([]string, error) {
	layers, err := ll.layers()
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(ll.subPath, "/_layers") {
		return ll.children("sha256"), nil
	}
	if strings.HasSuffix(ll.subPath, "/sha256") {
		return ll.children(layers...), nil
	}
	sha := ll.subPath[strings.LastIndex(ll.subPath, "/")+1:]
	for _, l := range layers {
		if l == sha {
			return ll.children("link"), nil
		}
	}
	return nil, storagedriver.PathNotFoundError{Path: ll.path()}
}

type manifestList struct {
	filePath
	repo string
}

func (ml *manifestList) manifests() ([]string, error) {
	return ml.store.listRepoRevisions(ml.repo)
}

func (ml *manifestList) Reader() (io.ReadCloser, error) {
	return nil, errors.New("is a directory")
}

func (ml *manifestList) Stat() (storagedriver.FileInfo, error) {
	if _, err := ml.manifests(); err != nil {
		return nil, err
	}
	return storagedriver.FileInfoInternal{
		storagedriver.FileInfoFields{
			Path:  ml.path(),
			IsDir: true,
		},
	}, nil
}

func (ml *manifestList) List() ([]string, error) {
	manifests, err := ml.manifests()
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(ml.subPath, "/_manifests") {
		return ml.children("revisions"), nil
	}
	if strings.HasSuffix(ml.subPath, "/revisions") {
		return ml.children("sha256"), nil
	}
	if strings.HasSuffix(ml.subPath, "/sha256") {
		return ml.children(manifests...), nil
	}
	sha := ml.subPath[strings.LastIndex(ml.subPath, "/")+1:]
	for _, m := range manifests {
		if m == sha {
			return ml.children("link"), nil
		}
	}
	return nil, storagedriver.PathNotFoundError{Path: ml.path()}
}

type link struct {
	filePath
}

func (l *link) linkDigest() (string, error) {
	path := strings.Split(l.subPath, "/")
	if len(path) < 5 ||
		path[len(path)-1] != "link" ||
		path[len(path)-3] != "sha256" ||
		path[0] != "repositories" {
		return "", storagedriver.PathNotFoundError{Path: l.path()}
	}
	repoEnd := len(path) - 4
	switch path[repoEnd] {
	case "_layers":
		// TODO: check for layer existence
	case "revisions":
		repoEnd -= 1
		if path[repoEnd] != "_manifests" {
			return "", storagedriver.PathNotFoundError{Path: l.path()}
		}
		// TODO: check for manifest existence
	default:
		return "", storagedriver.PathNotFoundError{Path: l.path()}
	}
	return path[len(path)-2], nil
}

func (l *link) Reader() (io.ReadCloser, error) {
	digest, err := l.linkDigest()
	if err != nil {
		return nil, err
	}
	content := []byte(fmt.Sprintf("sha256:%s", digest))
	return io.NopCloser(bytes.NewReader(content)), nil
}

func (l *link) Stat() (storagedriver.FileInfo, error) {
	_, err := l.linkDigest()
	if err != nil {
		return nil, err
	}
	return storagedriver.FileInfoInternal{
		storagedriver.FileInfoFields{
			Path: l.path(),
			Size: 64 + 7,
		},
	}, nil
}

func (l *link) List() ([]string, error) {
	return nil, errors.New("not a directory")
}
