package registry

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"

	"github.com/containers/image/docker"
	"github.com/containers/image/types"
)

var tags []string

var ref types.ImageReference
var img types.Image
var imgSrc types.ImageSource
var allFiles map[string]*tar.Header

func init() {
	ref, _ = docker.ParseReference("//gcr.io/google-appengine/debian8")
	img, _ = ref.NewImage(nil)
	imgSrc, _ = ref.NewImageSource(nil, nil)
}

func Manifest() ([]byte, error) {
	contents, _, err := img.Manifest()
	return contents, err
}

func Tags() ([]string, error) {
	var err error
	if tags != nil {
		return tags, nil
	}

	dockerImg, ok := img.(*docker.Image)
	if !ok {
		return nil, fmt.Errorf("Unable to convert to Docker image.")
	}

	tags, err = dockerImg.GetRepositoryTags()
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func GetFileFromBlob(name string, digest types.BlobInfo) ([]byte, error) {
	b, _, err := imgSrc.GetBlob(digest)
	if err != nil {
		return nil, err
	}
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
	blobs := img.LayerInfos()
	fmt.Println("GetFileFromImage: ", name)
	for i := len(blobs) - 1; i >= 0; i-- {
		blob := blobs[i]
		f, err := GetFileFromBlob(name, blob)
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

	blobs := img.LayerInfos()
	for _, b := range blobs {
		bi, _, _ := imgSrc.GetBlob(b)
		gzf, err := gzip.NewReader(bi)
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
