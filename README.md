# RSSFS: rss feed as a fuse file system

Taken the idea and almost all semantics from this project: https://github.com/dertuxmalwieder/rssfs/
Completely rewritten for performance reasons (simply can't handle >100 feeds lol).

## How does it work?

Program creates for RSS feed a directory:
```
/mnt/rssfs/Open Source Feed/
```
And in this directory there are files which contain rss entries of this feed:
```
/mnt/rssfs/Open Source Feed/Hello World.html
/mnt/rssfs/Open Source Feed/Second Article.html
```

Every time you `ls` feed directory it updates feed entries (may be a bit slow).

## Install
```
go install github.com/i1i1/rssfs
```
After that you should create `rssfs.hcl` file in your `$XDG_CONFIG_HOME` directory. Checkout [example](./rssfs.hcl).
