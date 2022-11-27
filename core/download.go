package core

import (
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/schollz/progressbar/v3"
)

var Process int = 6

func DownloadHandle(url string, path string, parallel bool) {

	head, err := http.Head(url)
	if err != nil {
		log.Fatal(err)
	}

	path = PathHandle(path, *head)

	if parallel {
		if head.Header.Get("Accept-Ranges") == "bytes" && head.ContentLength != -1 {
			seperate := bytesSeperator(Process, int(head.ContentLength))
			prepareParallel(url, path, seperate, int(head.ContentLength))
		} else {
			log.Println("Couldn't use parallel download, switch to normal")
			normalDownload(url, path)
		}
	} else {
		normalDownload(url, path)

	}

}

func PathHandle(path string, head http.Response) string {
	var destination string
	if path == "" {
		if name := head.Header.Get("Content-Disposition"); name != "" {
			destination = strings.Split(name, "filename=")[1] // Support by website
		} else {
			destination = head.Request.URL.Path[strings.LastIndex(head.Request.URL.Path, "/")+1:] // Get last word of url
		}
	} else {
		if validateDirectory(path) {
			destination = path
		} else {
			log.Fatal("It's a directory")
		}
	}
	return destination
}

func validateDirectory(destination string) bool {
	inf, err := os.Stat(destination)
	return !(err == nil && inf.IsDir())
}

func normalDownload(url string, path string) {
	var resp http.Response

	{
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Fatal(err)
		}
		tempresp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		resp = *tempresp
	}

	file, err := os.Create(path)

	if err != nil {
		resp.Body.Close()
		log.Fatal(err)
	}
	bar := progressbar.DefaultBytes(resp.ContentLength, path)
	io.Copy(io.MultiWriter(file, bar), resp.Body)
	bar.Close()
	file.Close()
	resp.Body.Close()
}

func bytesSeperator(process int, length int) []string {

	bytesSeperate := make([]string, process)

	// use Arithmetic progression to caculate the position of requests' byte
	// Information https://en.wikipedia.org/wiki/Arithmetic_progression
	d := int(math.Floor(float64(length) / float64(process)))
	for i := 0; i < process; i++ {
		start := 0 + d*i
		end := d - 1 + i*d
		if i != process-1 {
			bytesSeperate[i] = fmt.Sprintf("%d-%d", start, end)
		} else {
			bytesSeperate[i] = fmt.Sprintf("%d-", start)
		}
	}
	return bytesSeperate
}

func prepareParallel(url string, path string, bytesRange []string, contentLength int) {

	tempfileList := make([]string, Process)

	dir, err := os.MkdirTemp(os.TempDir(), "BGD")
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(Process)

	bar := progressbar.DefaultBytes(int64(contentLength), "Download")

	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}

	for i, index := range bytesRange {
		tempfileList[i] = filepath.Join(dir, "BGD-"+strconv.Itoa(i)+".dtemp")
		go parallelDownload(url, tempfileList[i], bar, &wg, index)
	}
	wg.Wait()

	for _, index := range tempfileList {
		file, _ := os.OpenFile(index, os.O_RDONLY, 0777)
		io.Copy(f, file)
		file.Close()
	}
	if err := os.RemoveAll(dir); err != nil {
		log.Fatal(err)
	}

}

func parallelDownload(url string, path string, bar *progressbar.ProgressBar, wg *sync.WaitGroup, bytesRange string) {
	var resp http.Response

	{
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Set("Range", "bytes="+bytesRange)
		tempresp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		resp = *tempresp
	}

	file, err := os.Create(path)

	if err != nil {
		resp.Body.Close()
		log.Fatal(err)
	}

	if resp.StatusCode != 206 {
		log.Fatal("HTTP ", resp.StatusCode, " does not support parallel")
	}

	io.Copy(io.MultiWriter(file, bar), resp.Body)
	bar.Close()
	file.Close()
	resp.Body.Close()
	wg.Done()
}
