package driver

import (
	"errors"
	"fmt"
	"io"
	"strings"

	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"
)

type blobList struct {
	filePath
}

func (bl *blobList) Reader() (io.ReadCloser, error) {
	return nil, errors.New("is a directory")
}

func (bl *blobList) Stat() (storagedriver.FileInfo, error) {
	info := storagedriver.FileInfoInternal{
		storagedriver.FileInfoFields{
			Path:  bl.path(),
			IsDir: true,
		},
	}

	path := strings.Split(bl.subPath, "/")
	if path[0] != "blobs" {
		return nil, storagedriver.PathNotFoundError{Path: bl.path()}
	}
	if len(path) == 1 {
		return info, nil
	}

	if path[1] != "sha256" {
		return nil, storagedriver.PathNotFoundError{Path: bl.path()}
	}
	if len(path) == 2 {
		return info, nil
	}
	blobs, err := bl.store.listBlobs()
	if err != nil {
		return nil, err
	}
	if len(path) == 3 {
		for _, sha := range blobs {
			if sha[:2] == path[2] {
				return info, nil
			}
		}
	}
	if path[3][:2] != path[2] {
		return nil, storagedriver.PathNotFoundError{Path: bl.path()}
	}
	if len(path) == 4 {
		for _, sha := range blobs {
			if sha == path[3] {
				return info, nil
			}
		}
	}
	return nil, storagedriver.PathNotFoundError{Path: bl.path()}
}

func (bl *blobList) List() ([]string, error) {
	path := strings.Split(bl.subPath, "/")
	if path[0] != "blobs" {
		return nil, storagedriver.PathNotFoundError{Path: bl.path()}
	}
	if len(path) == 1 {
		return bl.children("sha256"), nil
	}

	if path[1] != "sha256" {
		return nil, storagedriver.PathNotFoundError{Path: bl.path()}
	}
	blobs, err := bl.store.listBlobs()
	if err != nil {
		return nil, err
	}
	if len(path) == 2 {
		dirs := map[string]struct{}{}
		for _, sha := range blobs {
			dirs[sha[:2]] = struct{}{}
		}
		dirlist := make([]string, 0, len(dirs))
		for k, _ := range dirs {
			dirlist = append(dirlist, k)
		}
		return bl.children(dirlist...), nil
	}
	if len(path) == 3 {
		dirlist := []string{}
		for _, sha := range blobs {
			if sha[:2] == path[2] {
				dirlist = append(dirlist, sha)
			}
		}
		return bl.children(dirlist...), nil
	}
	if path[3][:2] != path[2] {
		return nil, storagedriver.PathNotFoundError{Path: bl.path()}
	}
	if len(path) == 4 {
		for _, sha := range blobs {
			if sha == path[3] {
				return bl.children("data"), nil
			}
		}
	}
	return nil, storagedriver.PathNotFoundError{Path: bl.path()}
}

type blob struct {
	filePath
}

func (b *blob) getBlob() (blobFunc, int64, error) {
	path := strings.Split(b.subPath, "/")
	if len(path) != 5 ||
		path[1] != "sha256" ||
		path[3][:2] != path[2] ||
		path[4] != "data" {
		return nil, 0, storagedriver.PathNotFoundError{Path: b.path()}
	}
	return b.store.getBlob(path[3])
}

func (b *blob) Reader() (io.ReadCloser, error) {
	getBlobReader, _, err := b.getBlob()
	if err != nil {
		return nil, err
	}
	return getBlobReader()
}

func (b *blob) Stat() (storagedriver.FileInfo, error) {
	_, size, err := b.getBlob()
	if err != nil {
		return nil, fmt.Errorf("cannot stat blob: %w", err)
		//return nil, err
	}
	return storagedriver.FileInfoInternal{
		storagedriver.FileInfoFields{
			Path: b.path(),
			Size: size,
		},
	}, nil
}

func (b *blob) List() ([]string, error) {
	return nil, errors.New("not a directory")
}
