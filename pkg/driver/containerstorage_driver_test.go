package driver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	storagedriver "github.com/distribution/distribution/v3/registry/storage/driver"
	"github.com/stretchr/testify/assert"
)

const testRepo = "foo/bar"

type fakeStore struct{}

func (fs fakeStore) listRepos() ([]string, error) {
	return []string{testRepo}, nil
}

func (fs fakeStore) listRepoRevisions(repo string) ([]string, error) {
	if repo != testRepo {
		return nil, fmt.Errorf("non-existent repo %v", repo)
	}
	return []string{
		"e9b1ebd668736b15a9c564b21d228266365144ab84ff83efd4fbd0dbf48cf270",
	}, nil
}

func (fs fakeStore) listRepoLayers(repo string) ([]string, error) {
	if repo != testRepo {
		return nil, fmt.Errorf("non-existent repo %v", repo)
	}
	return []string{
		"011825408f0fa194be09306dd9a780139c84113d9854e8df169f0f36a2b767d1",
		"0d557d32f54ebd277fdffbbdf656b90442ee9d8753aec9ebac429eee967f4dee",
		"17facd475902d6709cff908630b59271c7ad18f64c3a1d0143d438c6988504ef",
		"1c7514c910aedd7bf057cfd77022242c2aeed353fd069d9e0737115279b81945",
		"2ebbb2a31926293c8b7c870a6573f6136b039b3c74dd7c03b565dfb96707084d",
		"45af84228eb6d0dc1507484ed66b7412df1ff7612529e8f0bd276fd2e895eabf",
		"537c3ac04d51420a15dd455065e491a4fbc8a64fc90d5cc2c4f4d3bc7f03639f",
		"57ecce25721efcc305451de08105f847a9b7f9abde7a607693c4c6eca805ca0e",
		"580aadba0734e53f6a9f99e4d35952fbb1a5996cd056d6cac9f7c72cf9dda78a",
		"6c5de04c936da27e33992af1e54e929f1cb39c8e1473d9d25ed1f1dc2d842fd4",
		"73b199b6a14c15a166d3855f9ca4eb18ae2ba2ae2fe4c5efbbe9759b69ec61bd",
		"850b42373d0247bcc11d75c163e1347b0c33178124080a96aa9f11645514c9ad",
		"9795cafca922075b1e0fba7e3ef43324548c24d8fcf7afab6bfbe1285fcc8644",
		"f1ee40d9db4a2bf9b96ea48d6cb45c602a6761650f67dc84bba5a0d2495e845a",
		"fac57c834659f6777660e4158adb396cdbc16054000b123805eae5472a3874fa",
	}, nil
}

func (fs fakeStore) listBlobs() ([]string, error) {
	revs, err := fs.listRepoRevisions(testRepo)
	if err != nil {
		return nil, err
	}
	layers, err := fs.listRepoLayers(testRepo)
	if err != nil {
		return nil, err
	}
	return append(revs, layers...), nil
}

func (fs fakeStore) getBlob(sha string) (io.ReadCloser, int64, error) {
	blobs, err := fs.listBlobs()
	if err != nil {
		return nil, 0, err
	}
	for _, b := range blobs {
		if sha == b {
			return io.NopCloser(bytes.NewReader([]byte("Hello, World!"))), 13, nil
		}
	}
	return nil, 0, fmt.Errorf("non-existent blob %v", sha)
}

const expectedFiles = `/docker
/docker/registry
/docker/registry/v2
/docker/registry/v2/blobs
/docker/registry/v2/blobs/sha256
/docker/registry/v2/blobs/sha256/01
/docker/registry/v2/blobs/sha256/01/011825408f0fa194be09306dd9a780139c84113d9854e8df169f0f36a2b767d1
/docker/registry/v2/blobs/sha256/01/011825408f0fa194be09306dd9a780139c84113d9854e8df169f0f36a2b767d1/data
/docker/registry/v2/blobs/sha256/0d
/docker/registry/v2/blobs/sha256/0d/0d557d32f54ebd277fdffbbdf656b90442ee9d8753aec9ebac429eee967f4dee
/docker/registry/v2/blobs/sha256/0d/0d557d32f54ebd277fdffbbdf656b90442ee9d8753aec9ebac429eee967f4dee/data
/docker/registry/v2/blobs/sha256/17
/docker/registry/v2/blobs/sha256/17/17facd475902d6709cff908630b59271c7ad18f64c3a1d0143d438c6988504ef
/docker/registry/v2/blobs/sha256/17/17facd475902d6709cff908630b59271c7ad18f64c3a1d0143d438c6988504ef/data
/docker/registry/v2/blobs/sha256/1c
/docker/registry/v2/blobs/sha256/1c/1c7514c910aedd7bf057cfd77022242c2aeed353fd069d9e0737115279b81945
/docker/registry/v2/blobs/sha256/1c/1c7514c910aedd7bf057cfd77022242c2aeed353fd069d9e0737115279b81945/data
/docker/registry/v2/blobs/sha256/2e
/docker/registry/v2/blobs/sha256/2e/2ebbb2a31926293c8b7c870a6573f6136b039b3c74dd7c03b565dfb96707084d
/docker/registry/v2/blobs/sha256/2e/2ebbb2a31926293c8b7c870a6573f6136b039b3c74dd7c03b565dfb96707084d/data
/docker/registry/v2/blobs/sha256/45
/docker/registry/v2/blobs/sha256/45/45af84228eb6d0dc1507484ed66b7412df1ff7612529e8f0bd276fd2e895eabf
/docker/registry/v2/blobs/sha256/45/45af84228eb6d0dc1507484ed66b7412df1ff7612529e8f0bd276fd2e895eabf/data
/docker/registry/v2/blobs/sha256/53
/docker/registry/v2/blobs/sha256/53/537c3ac04d51420a15dd455065e491a4fbc8a64fc90d5cc2c4f4d3bc7f03639f
/docker/registry/v2/blobs/sha256/53/537c3ac04d51420a15dd455065e491a4fbc8a64fc90d5cc2c4f4d3bc7f03639f/data
/docker/registry/v2/blobs/sha256/57
/docker/registry/v2/blobs/sha256/57/57ecce25721efcc305451de08105f847a9b7f9abde7a607693c4c6eca805ca0e
/docker/registry/v2/blobs/sha256/57/57ecce25721efcc305451de08105f847a9b7f9abde7a607693c4c6eca805ca0e/data
/docker/registry/v2/blobs/sha256/58
/docker/registry/v2/blobs/sha256/58/580aadba0734e53f6a9f99e4d35952fbb1a5996cd056d6cac9f7c72cf9dda78a
/docker/registry/v2/blobs/sha256/58/580aadba0734e53f6a9f99e4d35952fbb1a5996cd056d6cac9f7c72cf9dda78a/data
/docker/registry/v2/blobs/sha256/6c
/docker/registry/v2/blobs/sha256/6c/6c5de04c936da27e33992af1e54e929f1cb39c8e1473d9d25ed1f1dc2d842fd4
/docker/registry/v2/blobs/sha256/6c/6c5de04c936da27e33992af1e54e929f1cb39c8e1473d9d25ed1f1dc2d842fd4/data
/docker/registry/v2/blobs/sha256/73
/docker/registry/v2/blobs/sha256/73/73b199b6a14c15a166d3855f9ca4eb18ae2ba2ae2fe4c5efbbe9759b69ec61bd
/docker/registry/v2/blobs/sha256/73/73b199b6a14c15a166d3855f9ca4eb18ae2ba2ae2fe4c5efbbe9759b69ec61bd/data
/docker/registry/v2/blobs/sha256/85
/docker/registry/v2/blobs/sha256/85/850b42373d0247bcc11d75c163e1347b0c33178124080a96aa9f11645514c9ad
/docker/registry/v2/blobs/sha256/85/850b42373d0247bcc11d75c163e1347b0c33178124080a96aa9f11645514c9ad/data
/docker/registry/v2/blobs/sha256/97
/docker/registry/v2/blobs/sha256/97/9795cafca922075b1e0fba7e3ef43324548c24d8fcf7afab6bfbe1285fcc8644
/docker/registry/v2/blobs/sha256/97/9795cafca922075b1e0fba7e3ef43324548c24d8fcf7afab6bfbe1285fcc8644/data
/docker/registry/v2/blobs/sha256/e9
/docker/registry/v2/blobs/sha256/e9/e9b1ebd668736b15a9c564b21d228266365144ab84ff83efd4fbd0dbf48cf270
/docker/registry/v2/blobs/sha256/e9/e9b1ebd668736b15a9c564b21d228266365144ab84ff83efd4fbd0dbf48cf270/data
/docker/registry/v2/blobs/sha256/f1
/docker/registry/v2/blobs/sha256/f1/f1ee40d9db4a2bf9b96ea48d6cb45c602a6761650f67dc84bba5a0d2495e845a
/docker/registry/v2/blobs/sha256/f1/f1ee40d9db4a2bf9b96ea48d6cb45c602a6761650f67dc84bba5a0d2495e845a/data
/docker/registry/v2/blobs/sha256/fa
/docker/registry/v2/blobs/sha256/fa/fac57c834659f6777660e4158adb396cdbc16054000b123805eae5472a3874fa
/docker/registry/v2/blobs/sha256/fa/fac57c834659f6777660e4158adb396cdbc16054000b123805eae5472a3874fa/data
/docker/registry/v2/repositories
/docker/registry/v2/repositories/foo/bar
/docker/registry/v2/repositories/foo/bar/_layers
/docker/registry/v2/repositories/foo/bar/_layers/sha256
/docker/registry/v2/repositories/foo/bar/_layers/sha256/011825408f0fa194be09306dd9a780139c84113d9854e8df169f0f36a2b767d1
/docker/registry/v2/repositories/foo/bar/_layers/sha256/011825408f0fa194be09306dd9a780139c84113d9854e8df169f0f36a2b767d1/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/0d557d32f54ebd277fdffbbdf656b90442ee9d8753aec9ebac429eee967f4dee
/docker/registry/v2/repositories/foo/bar/_layers/sha256/0d557d32f54ebd277fdffbbdf656b90442ee9d8753aec9ebac429eee967f4dee/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/17facd475902d6709cff908630b59271c7ad18f64c3a1d0143d438c6988504ef
/docker/registry/v2/repositories/foo/bar/_layers/sha256/17facd475902d6709cff908630b59271c7ad18f64c3a1d0143d438c6988504ef/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/1c7514c910aedd7bf057cfd77022242c2aeed353fd069d9e0737115279b81945
/docker/registry/v2/repositories/foo/bar/_layers/sha256/1c7514c910aedd7bf057cfd77022242c2aeed353fd069d9e0737115279b81945/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/2ebbb2a31926293c8b7c870a6573f6136b039b3c74dd7c03b565dfb96707084d
/docker/registry/v2/repositories/foo/bar/_layers/sha256/2ebbb2a31926293c8b7c870a6573f6136b039b3c74dd7c03b565dfb96707084d/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/45af84228eb6d0dc1507484ed66b7412df1ff7612529e8f0bd276fd2e895eabf
/docker/registry/v2/repositories/foo/bar/_layers/sha256/45af84228eb6d0dc1507484ed66b7412df1ff7612529e8f0bd276fd2e895eabf/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/537c3ac04d51420a15dd455065e491a4fbc8a64fc90d5cc2c4f4d3bc7f03639f
/docker/registry/v2/repositories/foo/bar/_layers/sha256/537c3ac04d51420a15dd455065e491a4fbc8a64fc90d5cc2c4f4d3bc7f03639f/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/57ecce25721efcc305451de08105f847a9b7f9abde7a607693c4c6eca805ca0e
/docker/registry/v2/repositories/foo/bar/_layers/sha256/57ecce25721efcc305451de08105f847a9b7f9abde7a607693c4c6eca805ca0e/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/580aadba0734e53f6a9f99e4d35952fbb1a5996cd056d6cac9f7c72cf9dda78a
/docker/registry/v2/repositories/foo/bar/_layers/sha256/580aadba0734e53f6a9f99e4d35952fbb1a5996cd056d6cac9f7c72cf9dda78a/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/6c5de04c936da27e33992af1e54e929f1cb39c8e1473d9d25ed1f1dc2d842fd4
/docker/registry/v2/repositories/foo/bar/_layers/sha256/6c5de04c936da27e33992af1e54e929f1cb39c8e1473d9d25ed1f1dc2d842fd4/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/73b199b6a14c15a166d3855f9ca4eb18ae2ba2ae2fe4c5efbbe9759b69ec61bd
/docker/registry/v2/repositories/foo/bar/_layers/sha256/73b199b6a14c15a166d3855f9ca4eb18ae2ba2ae2fe4c5efbbe9759b69ec61bd/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/850b42373d0247bcc11d75c163e1347b0c33178124080a96aa9f11645514c9ad
/docker/registry/v2/repositories/foo/bar/_layers/sha256/850b42373d0247bcc11d75c163e1347b0c33178124080a96aa9f11645514c9ad/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/9795cafca922075b1e0fba7e3ef43324548c24d8fcf7afab6bfbe1285fcc8644
/docker/registry/v2/repositories/foo/bar/_layers/sha256/9795cafca922075b1e0fba7e3ef43324548c24d8fcf7afab6bfbe1285fcc8644/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/f1ee40d9db4a2bf9b96ea48d6cb45c602a6761650f67dc84bba5a0d2495e845a
/docker/registry/v2/repositories/foo/bar/_layers/sha256/f1ee40d9db4a2bf9b96ea48d6cb45c602a6761650f67dc84bba5a0d2495e845a/link
/docker/registry/v2/repositories/foo/bar/_layers/sha256/fac57c834659f6777660e4158adb396cdbc16054000b123805eae5472a3874fa
/docker/registry/v2/repositories/foo/bar/_layers/sha256/fac57c834659f6777660e4158adb396cdbc16054000b123805eae5472a3874fa/link
/docker/registry/v2/repositories/foo/bar/_manifests
/docker/registry/v2/repositories/foo/bar/_manifests/revisions
/docker/registry/v2/repositories/foo/bar/_manifests/revisions/sha256
/docker/registry/v2/repositories/foo/bar/_manifests/revisions/sha256/e9b1ebd668736b15a9c564b21d228266365144ab84ff83efd4fbd0dbf48cf270
/docker/registry/v2/repositories/foo/bar/_manifests/revisions/sha256/e9b1ebd668736b15a9c564b21d228266365144ab84ff83efd4fbd0dbf48cf270/link
`

func TestWalk(t *testing.T) {
	d := driver{
		store: fakeStore{},
	}
	files, err := func() ([]string, error) {
		files := []string{}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()
		err := d.Walk(ctx,
			"/", func(fileInfo storagedriver.FileInfo) error {
				files = append(files, fileInfo.Path()+"\n")
				return nil
			})
		return files, err
	}()
	assert.NoError(t, err)
	sort.Strings(files)
	assert.Equal(t, expectedFiles, strings.Join(files, ""))
}
