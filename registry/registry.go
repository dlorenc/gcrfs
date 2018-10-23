package registry

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/google"

	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var tags []string

var img v1.Image
var ref name.Reference
var allFiles map[string]*tar.Header

func init() {
	ref, _ = name.ParseReference("gcr.io/google-appengine/debian9:latest", name.WeakValidation)
	img, _ = remote.Image(ref)
}

func Manifest() ([]byte, error) {
	return img.RawManifest()
}

func Tags() ([]string, error) {
	r, _ := name.NewRepository("gcr.io/google-appengine/debian9", name.WeakValidation)
	tags, err := google.List(r, authn.Anonymous, http.DefaultTransport)
	if err != nil {
		return nil, err
	}
	return tags.Tags, nil
}

func GetFileFromBlob(name string, digest v1.Hash) ([]byte, error) {
	l, _ := img.LayerByDigest(digest)
	b, _ := l.Compressed()
	gzf, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}
	tf := tar.NewReader(gzf)
	for header, err := tf.Next(); err != io.EOF; header, err = tf.Next() {
		switch header.Typeflag {
		case tar.TypeReg:
			if header.Name == name {
				data := make([]byte, header.Size)
				_, err := tf.Read(data)
				if err != nil {
					return nil, err
				}
				return data, nil
			}
		}
	}
	return nil, nil
}

func GetFileFromImage(name string) ([]byte, error) {
	blobs, _ := img.BlobSet()
	fmt.Println("GetFileFromImage: ", name)
	for b := range blobs {
		f, err := GetFileFromBlob(name, b)
		if err != nil {
			return nil, err
		}
		if f != nil {
			return f, nil
		}
	}
	return nil, fmt.Errorf("File not found")
}

func cacheAllFiles() error {
	allFiles = make(map[string]*tar.Header)

	blobs, _ := img.BlobSet()
	for b := range blobs {
		l, _ := img.LayerByDigest(b)
		uc, _ := l.Compressed()
		gzf, err := gzip.NewReader(uc)
		if err != nil {
			return err
		}
		tf := tar.NewReader(gzf)
		for header, err := tf.Next(); err != io.EOF; header, err = tf.Next() {
			allFiles["/"+filepath.Clean(header.Name)] = header
		}
	}
	return nil
}

func GetFilesFromImage(path string) (map[string]*tar.Header, error) {
	fmt.Println("GetFiles: ", path)
	if allFiles == nil {
		cacheAllFiles()
	}

	files := map[string]*tar.Header{}
	for p, h := range allFiles {
		dir, _ := filepath.Split(filepath.Clean(p))
		if filepath.Clean(dir) == path {
			fmt.Println("Found one! ", p)
			files[p] = h
		}
	}

	return files, nil
}

func GetFileAttrFromImage(path string) *tar.Header {
	if allFiles == nil {
		cacheAllFiles()
	}
	return allFiles[path]
}
