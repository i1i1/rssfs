package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type (
	node interface {
		node() *GenericNode
		size() uint64
		mode() uint32
	}

	dirnode interface {
		node
		readdir() []node
		lookup(name string) (node, bool)
	}

	GenericNode struct {
		fs.Inode
		Ino       uint64
		Filename  string
		Timestamp time.Time
	}

	RootNode struct {
		GenericNode
		Cats *sync.Map // map[string]*CategoryNode
	}

	CategoryNode struct {
		GenericNode
		Feeds *sync.Map // map[string]*FeedNode
	}

	FeedNode struct {
		GenericNode
		Feed
		News *sync.Map // map[string]*NewsNode
	}

	NewsNode struct {
		GenericNode
		Data []byte
	}
)

var root RootNode

func (g *GenericNode) node() *GenericNode { return g }

func (dir *GenericNode) size() uint64 { return 0 }
func (dir *GenericNode) mode() uint32 { return 0755 | uint32(syscall.S_IFDIR) }

func (file *NewsNode) size() uint64 { return uint64(len(file.Data)) }
func (file *NewsNode) mode() uint32 { return 0644 | uint32(syscall.S_IFREG) }

func setAttributes(n node, out *fuse.Attr) {
	gn := n.node()
	user, err := user.Current()
	die(err)

	out.SetTimes(&gn.Timestamp, &gn.Timestamp, &gn.Timestamp)

	out.Mode = n.mode()
	out.Size = n.size()
	out.Ino = gn.Ino

	uid32, _ := strconv.ParseUint(user.Uid, 10, 32)
	gid32, _ := strconv.ParseUint(user.Gid, 10, 32)
	out.Uid = uint32(uid32)
	out.Gid = uint32(gid32)
}

func (gn *GenericNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	setAttributes(gn, &out.Attr)
	return fs.OK
}

func (nn *NewsNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	setAttributes(nn, &out.Attr)
	return fs.OK
}

func (root *RootNode) readdir() []node {
	ret := make([]node, 0)
	root.Cats.Range(func(_, v interface{}) bool {
		cat := v.(*CategoryNode)
		ret = append(ret, cat)
		return true
	})
	return ret
}

func (cn *CategoryNode) readdir() []node {
	ret := make([]node, 0)
	cn.Feeds.Range(func(_, v interface{}) bool {
		feed := v.(*FeedNode)
		ret = append(ret, feed)
		return true
	})
	return ret
}

func (fn *FeedNode) readdir() []node {
	fn.News = &sync.Map{}
	ret := make([]node, 0)

	feeds := GetFeedFiles(fn)
	for i := 0; i < len(feeds); i++ {
		fn.News.Store(feeds[i].Filename, &feeds[i])
		ret = append(ret, &feeds[i])
	}

	return ret
}

func nodesToDirstream(nodes []node, ctx context.Context) (fs.DirStream, syscall.Errno) {
	entries := make([]fuse.DirEntry, len(nodes))
	for i, node := range nodes {
		gn := node.node()
		entries[i] = fuse.DirEntry{
			Name: gn.Filename,
			Mode: gn.mode(),
			Ino:  gn.Ino,
		}
	}
	ds := fs.NewListDirStream(entries)
	return ds, fs.OK
}

// Readdir returns a list of file entries for currentPath().
func (fn *FeedNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return nodesToDirstream(fn.readdir(), ctx)
}
func (cn *CategoryNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return nodesToDirstream(cn.readdir(), ctx)
}
func (root *RootNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	return nodesToDirstream(root.readdir(), ctx)
}

func dirlookup(dn dirnode, ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	n, ok := dn.lookup(name)
	if !ok {
		return nil, syscall.ENOENT
	}

	var child *fs.Inode

	dir := dn.node()

	switch v := n.(type) {
	case *CategoryNode:
		setAttributes(v, &out.Attr)
		sa := fs.StableAttr{
			Mode: v.mode(),
			Gen:  1,
			Ino:  v.Ino,
		}
		child = dir.NewPersistentInode(ctx, v, sa)
	case *FeedNode:
		setAttributes(n, &out.Attr)
		setAttributes(v, &out.Attr)
		sa := fs.StableAttr{
			Mode: v.mode(),
			Gen:  1,
			Ino:  v.Ino,
		}
		child = dir.NewPersistentInode(ctx, v, sa)
	case *NewsNode:
		setAttributes(v, &out.Attr)
		sa := fs.StableAttr{
			Mode: v.mode(),
			Gen:  1,
			Ino:  v.Ino,
		}
		child = dir.NewPersistentInode(ctx, v, sa)
	default:
		panic("Unknown type")
	}
	return child, fs.OK
}

func lookupSyncMap(sm *sync.Map, name string) (node, bool) {
	v, ok := sm.Load(name)
	if !ok {
		return nil, false
	}
	return v.(node), true
}

func (root *RootNode) lookup(name string) (node, bool) {
	return lookupSyncMap(root.Cats, name)
}
func (cn *CategoryNode) lookup(name string) (node, bool) {
	return lookupSyncMap(cn.Feeds, name)
}
func (fn *FeedNode) lookup(name string) (node, bool) {
	return lookupSyncMap(fn.News, name)
}

// Lookup generates a node for the given file.
func (fn *FeedNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	return dirlookup(fn, ctx, name, out)
}
func (cn *CategoryNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	return dirlookup(cn, ctx, name, out)
}
func (root *RootNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	return dirlookup(root, ctx, name, out)
}

// Read returns the requested file as bytes.
func (nn *NewsNode) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	end := int(off) + len(dest)
	if end > len(nn.Data) {
		end = len(nn.Data)
	}
	return fuse.ReadResultData(nn.Data[off:end]), fs.OK
}

func (nn *NewsNode) Flush(ctx context.Context, fh fs.FileHandle) syscall.Errno {
	return fs.OK
}

// Open parses the file into a file struct.
func (nn *NewsNode) Open(ctx context.Context, mode uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	return nn, 0, 0
}

func (f *FeedNode) Opendir(ctx context.Context) syscall.Errno     { return fs.OK }
func (c *CategoryNode) Opendir(ctx context.Context) syscall.Errno { return fs.OK }
func (_ *RootNode) Opendir(ctx context.Context) syscall.Errno     { return fs.OK }

func getFeedNode(f *Feed) (fn FeedNode) {
	feeddata := getFeedData(f.URL)
	return FeedNode{
		GenericNode{
			Timestamp: time.Now(),
			Filename:  feeddata.Title,
			Ino:       getInode([]byte("feed " + feeddata.Title)),
		},
		Feed{f.URL},
		&sync.Map{},
	}
}

func getCategoryNode(cat *Category) (cn CategoryNode) {
	cn = CategoryNode{
		GenericNode: GenericNode{
			Timestamp: time.Now(),
			Filename:  cat.Name,
			Ino:       getInode([]byte("Cat " + cat.Name)),
		},
		Feeds: &sync.Map{},
	}
	ch := make(chan FeedNode, 20)

	for _, f := range cat.Feeds {
		go func(ch chan<- FeedNode, f Feed) {
			ch <- getFeedNode(&f)
		}(ch, *f)
	}

	for range cat.Feeds {
		feed := <-ch
		feed.Filename = fileNameClean(feed.Filename)
		cn.Feeds.Store(feed.Filename, &feed)
	}
	close(ch)
	return
}

func getRootNode(cats []*Category) (root RootNode) {
	root = RootNode{
		GenericNode: GenericNode{Timestamp: time.Now()},
		Cats:        &sync.Map{},
	}
	ch := make(chan CategoryNode, 20)

	for _, c := range cats {
		go func(ch chan<- CategoryNode, c Category) {
			ch <- getCategoryNode(&c)
		}(ch, *c)
	}

	for range cats {
		cat := <-ch
		cat.Filename = fileNameClean(cat.Filename)
		root.Cats.Store(cat.Filename, &cat)
	}
	close(ch)
	return
}

func Mount(cfg RssfsConfig) {
	root = getRootNode(cfg.Categories)

	sec := time.Second
	opts := &fs.Options{
		AttrTimeout:  &sec,
		EntryTimeout: &sec,
		MountOptions: fuse.MountOptions{
			AllowOther: true,
			Debug:      false,
			FsName:     "rssfs",
		},
	}

	fs := fs.NewNodeFS(&root, opts)
	mountPoint := cfg.MountPoint

	server, err := fuse.NewServer(fs, mountPoint, &opts.MountOptions)
	die(err)

	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
		for {
			<-c
			fmt.Println("\nTrying to unmount...\n")
			go server.Unmount()
		}
	}()
	fmt.Println("Serving...")
	server.Serve()
}
