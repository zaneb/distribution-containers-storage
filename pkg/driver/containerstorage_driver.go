package driver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/distribution/distribution/v3/registry/storage/driver/base"
	"github.com/distribution/distribution/v3/registry/storage/driver/factory"
)

type containerstorageDriverFactory struct{}

func init() {
	factory.Register("containerstorage", containerstorageDriverFactory{})
}

func (containerstorageDriverFactory) Create(parameters map[string]interface{}) (storagedriver.StorageDriver, error) {
	store, err := newContainerStorage()
	if err != nil {
		return nil, err
	}
	return base.NewRegulator(&driver{
		store: store,
	}, 1), nil
}

type driver struct {
	store store
}

func (d *driver) Name() string {
	return "containerstorage"
}

type pseudoFile interface {
	Reader() (io.ReadCloser, error)
	Stat() (storagedriver.FileInfo, error)
	List() ([]string, error)
}

type filePath struct {
	subPath string
	store   store
}

func (fp *filePath) path() string {
	return fmt.Sprintf("/docker/registry/v2/%s", fp.subPath)
}

func (fp *filePath) children(child ...string) []string {
	paths := make([]string, 0, len(child))
	for _, c := range child {
		paths = append(paths,
			fmt.Sprintf("/docker/registry/v2/%s/%s", fp.subPath, c))
	}
	return paths
}

type dir struct {
	path []string
}

func (d *dir) Reader() (io.ReadCloser, error) {
	return nil, errors.New("is a directory")
}

func (d *dir) Stat() (storagedriver.FileInfo, error) {
	return storagedriver.FileInfoInternal{
		storagedriver.FileInfoFields{
			Path:  "/" + strings.Join(d.path, "/"),
			IsDir: true,
		},
	}, nil
}

func (d *dir) List() ([]string, error) {
	switch len(d.path) {
	case 1:
		if d.path[0] == "" {
			return []string{"/docker"}, nil
		} else {
			return []string{"/docker/registry"}, nil
		}
	case 2:
		return []string{"/docker/registry/v2"}, nil
	case 3:
		return []string{"/docker/registry/v2/blobs", "/docker/registry/v2/repositories"}, nil
	}
	return nil, storagedriver.PathNotFoundError{Path: strings.Join(d.path, "/")}
}

func (d *driver) getFile(ctx context.Context, path string) (pseudoFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	segments := strings.Split(strings.TrimLeft(path, "/"), "/")
	switch len(segments) {
	default:
		fallthrough
	case 3:
		if segments[2] != "v2" {
			return nil, storagedriver.PathNotFoundError{Path: path}
		}
		fallthrough
	case 2:
		if segments[1] != "registry" {
			return nil, storagedriver.PathNotFoundError{Path: path}
		}
		fallthrough
	case 1:
		if !(segments[0] == "" || segments[0] == "docker") {
			return nil, storagedriver.PathNotFoundError{Path: path}
		}
	}
	if len(segments) < 4 {
		return &dir{path: segments}, nil
	}
	file := filePath{
		store:   d.store,
		subPath: strings.Join(segments[3:], "/"),
	}
	switch segments[3] {
	case "blobs":
		if segments[len(segments)-1] == "data" {
			return &blob{file}, nil
		}
		return &blobList{file}, nil
	case "repositories":
		if segments[len(segments)-1] == "link" {
			return &link{file}, nil
		}
		if len(segments) < 5 {
			return &repoList{file}, nil
		}
		repoSegments := []string{segments[4]}
		for _, s := range segments[5:] {
			if strings.HasPrefix(s, "_") {
				break
			}
			repoSegments = append(repoSegments, s)
		}
		repoName := strings.Join(repoSegments, "/")
		if len(segments)-len(repoSegments) < 5 {
			return &repo{
				filePath: file,
				repo:     repoName,
			}, nil
		}
		switch segments[4+len(repoSegments)] {
		case "_manifests":
			return &manifestList{
				filePath: file,
				repo:     repoName,
			}, nil
		case "_layers":
			return &layerList{
				filePath: file,
				repo:     repoName,
			}, nil
		}
	}
	return nil, storagedriver.PathNotFoundError{Path: path}
}

func (d *driver) GetContent(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reader, err := d.Reader(ctx, path, 0)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(reader)
}

func (d *driver) Reader(ctx context.Context, path string, offset int64) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := d.getFile(ctx, path)
	if err != nil {
		return nil, err
	}

	r, err := f.Reader()
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if _, err := io.Copy(io.Discard, io.LimitReader(r, offset)); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (d *driver) Stat(ctx context.Context, subPath string) (storagedriver.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := d.getFile(ctx, subPath)
	if err != nil {
		return nil, err
	}

	return f.Stat()
}

func (d *driver) List(ctx context.Context, subPath string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := d.getFile(ctx, subPath)
	if err != nil {
		return nil, err
	}

	return f.List()
}

func (d *driver) URLFor(ctx context.Context, path string, options map[string]interface{}) (string, error) {
	return "", storagedriver.ErrUnsupportedMethod{}
}

func (d *driver) Walk(ctx context.Context, path string, f storagedriver.WalkFn) error {
	return storagedriver.WalkFallback(ctx, d, path, f)
}

func (d *driver) PutContent(ctx context.Context, path string, contents []byte) error {
	return storagedriver.ErrUnsupportedMethod{}
}

func (d *driver) Writer(ctx context.Context, subPath string, append bool) (storagedriver.FileWriter, error) {
	return nil, storagedriver.ErrUnsupportedMethod{}
}

func (d *driver) Move(ctx context.Context, sourcePath, destPath string) error {
	return storagedriver.ErrUnsupportedMethod{}
}

func (d *driver) Delete(ctx context.Context, subPath string) error {
	return storagedriver.ErrUnsupportedMethod{}
}
