package fs

import (
	"archive/tar"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/dlorenc/gcrfs/registry"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

type GcrFS struct {
	pathfs.FileSystem
}

type openDirHandler func([]string) (c []fuse.DirEntry, code fuse.Status)

var openDirRoutes = []struct {
	pattern string
	handler openDirHandler
}{
	{"^$", OpenDirRoot},
	{"^tags$", OpenDirTags},
	// {"^digests$", OpenDirDigests},
	{"^tags/(.*)/rootfs(/.*)?$", OpenDirRootfs},
	{"^tags/(.*)$", OpenDirTag},
}

func OpenDirRoot(name []string) ([]fuse.DirEntry, fuse.Status) {
	return []fuse.DirEntry{
		{Name: "tags", Mode: fuse.S_IFDIR},
		{Name: "digests", Mode: fuse.S_IFDIR},
	}, fuse.OK
}

func OpenDirTags(name []string) ([]fuse.DirEntry, fuse.Status) {
	tags, err := registry.Tags()
	if err != nil {
		return nil, fuse.EBADF
	}
	var c []fuse.DirEntry
	for _, t := range tags {
		c = append(c, fuse.DirEntry{
			Name: t,
			Mode: fuse.S_IFDIR,
		})
	}
	return c, fuse.OK
}

func OpenDirTag(name []string) ([]fuse.DirEntry, fuse.Status) {
	return []fuse.DirEntry{
		{Name: "manifest", Mode: fuse.S_IFREG},
		{Name: "rootfs", Mode: fuse.S_IFDIR},
	}, fuse.OK
}

func OpenDirRootfs(name []string) ([]fuse.DirEntry, fuse.Status) {
	// fullMatch is 0
	// tag is 1
	pathInRoot := filepath.Clean("/" + name[2])
	files, err := registry.GetFilesFromImage(pathInRoot)
	if err != nil {
		return nil, fuse.ENODATA
	}
	entries := []fuse.DirEntry{}
	for p, f := range files {
		// TODO handle links
		if f.Mode == tar.TypeDir {
			entries = append(entries, fuse.DirEntry{
				Mode: fuse.S_IFDIR,
				Name: p,
			})
		} else {
			entries = append(entries, fuse.DirEntry{
				Mode: fuse.S_IFREG,
				Name: p,
			})
		}
	}
	return entries, fuse.OK
}

func (me *GcrFS) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	for _, route := range openDirRoutes {
		x := regexp.MustCompile(route.pattern)
		match := x.FindStringSubmatch(name)
		if match != nil {
			return route.handler(match)
		}
	}

	fmt.Println("NO OPEN DIRMATCH FOR: ", name)
	return nil, fuse.ENOENT
}

type getAttrHandler func([]string) (*fuse.Attr, fuse.Status)

var getAttrRoutes = []struct {
	pattern string
	handler getAttrHandler
}{
	{"^$", GetAttrDir},
	{"^tags$", GetAttrDir},
	{"^digests$", GetAttrDir},
	{"^tags/.*/rootfs$", GetAttrDir},
	{"^tags/.*/rootfs/(.*)?$", GetAttrRootfs},
	{"^tags/.*/manifest$", GetAttrManifest},
	{"^tags/(.*)$", GetAttrTag},
}

func GetAttrDir(path []string) (*fuse.Attr, fuse.Status) {
	return &fuse.Attr{
		Mode: fuse.S_IFDIR | 0755,
	}, fuse.OK
}

func GetAttrManifest(path []string) (*fuse.Attr, fuse.Status) {
	manifest, err := registry.Manifest()
	if err != nil {
		return nil, fuse.EBADF
	}
	return &fuse.Attr{
		Mode: fuse.S_IFREG | 0644,
		Size: uint64(len(manifest)),
	}, fuse.OK
}

func GetAttrTag(path []string) (*fuse.Attr, fuse.Status) {
	tags, err := registry.Tags()
	if err != nil {
		return nil, fuse.ENOENT
	}

	tag := path[1]
	for _, t := range tags {
		if t == tag {
			return &fuse.Attr{
				Mode: fuse.S_IFDIR | 0755,
			}, fuse.OK
		}
	}
	return nil, fuse.ENOENT
}

func GetAttrRootfs(path []string) (*fuse.Attr, fuse.Status) {
	// full match is 0
	name := "/" + path[1]
	fmt.Println("GetattrRootfs: ", name)
	f := registry.GetFileAttrFromImage(name)
	if f == nil {
		return nil, fuse.EBADF
	}
	return &fuse.Attr{
		Mode: uint32(f.Mode),
		Size: uint64(f.Size),
	}, fuse.OK

}

func (me *GcrFS) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	fmt.Println("getattr: ", name)
	for _, route := range getAttrRoutes {
		x := regexp.MustCompile(route.pattern)
		match := x.FindStringSubmatch(name)
		if match != nil {
			return route.handler(match)
		}
	}

	fmt.Println("NO GETATTR MATCH FOR: ", name)
	return nil, fuse.ENOENT
}

type openHandler func([]string) (file nodefs.File, code fuse.Status)

var openRoutes = []struct {
	pattern string
	handler openHandler
}{
	{"^tags/.*?/manifest$", OpenManifest},
	{"^tags/(.*?)/rootfs/(.*)?$", OpenFile},
}

func OpenManifest([]string) (file nodefs.File, code fuse.Status) {
	m, err := registry.Manifest()
	if err != nil {
		return nil, fuse.EBADF
	}
	return nodefs.NewReadOnlyFile(nodefs.NewDataFile(m)), fuse.OK
}

func OpenFile(args []string) (nodefs.File, fuse.Status) {
	path := args[2]
	f, _ := registry.GetFileFromImage(path)
	return nodefs.NewReadOnlyFile(nodefs.NewDataFile(f)), fuse.OK
}

func (me *GcrFS) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	fmt.Println("Open: ", name)
	for _, route := range openRoutes {
		x := regexp.MustCompile(route.pattern)
		match := x.FindStringSubmatch(name)
		if match != nil {
			return route.handler(match)
		}
	}

	fmt.Println("NO OPEN MATCH FOR: ", name)
	return nil, fuse.ENOENT
}
