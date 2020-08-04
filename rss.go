package main

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

type ByTitle []*gofeed.Item

func (a ByTitle) Len() int      { return len(a) }
func (a ByTitle) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByTitle) Less(i, j int) bool {
	return fileNameClean(a[i].Title) < fileNameClean(a[j].Title)
}

func fileNameClean(in string) string {
	// Returns a valid file name for <in>.
	invalidFileNameCharacters := []string{"/", "\\", ":", "#", "?", "<", ">", "*", "|", "\""}
	for _, character := range invalidFileNameCharacters {
		in = strings.ReplaceAll(in, character, "-")
	}
	out := strings.TrimSpace(in)
	if len(out) > 256 {
		out = out[:200]
	}
	return out
}

func getItemTimestamp(item *gofeed.Item) time.Time {
	if item.UpdatedParsed != nil {
		return *(item.UpdatedParsed)
	}
	if item.PublishedParsed != nil {
		return *(item.PublishedParsed)
	}
	return time.Now()
}

func getInode(data []byte) uint64 {
	h := fnv.New64()
	_, err := h.Write(data)
	die(err)
	return h.Sum64()
}

func getFeedData(url string) *gofeed.Feed {
	feeddata, err := gofeed.NewParser().ParseURL(url)
	die(err)
	sort.Sort(ByTitle(feeddata.Items))
	return feeddata
}

func GetFeedFiles(f *FeedNode) []NewsNode {
	feeddata := getFeedData(f.URL)
	feedFiles := make([]NewsNode, 0)

	// Checking collisions
	title, prev_title := "", ""
	col_cnt := 0
	for _, item := range feeddata.Items {
		title = fileNameClean(item.Title)
		if title == prev_title {
			col_cnt += 1
		} else {
			col_cnt = 0
		}

		if col_cnt > 0 {
			title = fmt.Sprintf("%s [%d]", title, col_cnt)
		}

		prev_title = title
		extension, content := GenerateOutputData(item)
		fname := title + "." + extension

		feedFiles = append(feedFiles, NewsNode{
			GenericNode: GenericNode{
				Filename:  fname,
				Timestamp: getItemTimestamp(item),
				Ino:       getInode([]byte(content)),
			},
			Data: []byte(content),
		})
	}

	return feedFiles
}

func GenerateOutputData(it *gofeed.Item) (ext string, content string) {
	title := fmt.Sprintf("<h1><a href=\"%s\">%s</a></h1>", it.Link, it.Title)
	name := "Unknown author"
	if it.Author != nil {
		name = it.Author.Name
	} else {
		name = "Unknown author"
	}
	info := fmt.Sprintf("<h2><a href=\"%s\">%s</a> published at %v</h2>", it.Link, name, getItemTimestamp(it))
	content = fmt.Sprintf("%s\n%s\n%s", title, info, it.Content)
	return "html", content
}
