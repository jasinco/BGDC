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
	"time"

	"github.com/schollz/progressbar/v3"
)

func DownloadHandle(url string, path string, connections int) {

	tr := &http.Transport{MaxIdleConns: connections, IdleConnTimeout: 30 * time.Second, DisableCompression: false}
	client := &http.Client{Timeout: 20 * time.Second, Transport: tr}
	parallel, compressed, head := HeaderCheck(client, url)
	path = PathHandle(path, *head)
	log.Println("Parallel", parallel, "Compressed", compressed)

	if parallel && head.ContentLength != -1 {
		prepareParallel(client, url, path, compressed, bytesSeperator(connections, int(head.ContentLength)))
	} else {
		NormalDownload(client, url, path, compressed)
	}
}

func HeaderCheck(client *http.Client, url string) (bool, bool, *http.Response) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Accept-Encoding", "gzip")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	return resp.Header.Get("Accept-Ranges") == "bytes", resp.Header.Get("Content-Encoding") == "gzip", resp
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

func NormalDownload(client *http.Client, url string, path string, compressed bool) {
	req, _ := http.NewRequest("GET", url, nil)
	if compressed {
		req.Header.Add("Accept-Encoding", "gzip")
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		log.Fatal("HTTP Code ", resp.StatusCode, " Error ", err)
	}
	defer resp.Body.Close()
	log.Println("Saving to ", path)
	if f, err := os.Create(path); err == nil {
		progress := progressbar.Default(resp.ContentLength, "Downloading")
		io.Copy(io.MultiWriter(f, progress), resp.Body)
		progress.Close()
		f.Close()
	} else {
		log.Fatal(err)
	}
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

func prepareParallel(client *http.Client, url string, path string, compressed bool, brange []string) {
	tempdir, err := os.MkdirTemp(os.TempDir(), "BGDC-*")
	if err != nil {
		log.Fatal(err)
	}

	files := make([]string, len(brange))

	var wg sync.WaitGroup
	wg.Add(len(brange))

	for idx, i := range brange {
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Add("Range", "bytes="+i)
		if compressed {
			req.Header.Add("Accept-Encoding", "gzip")
		}
		files[idx] = filepath.Join(tempdir, "bgd-"+strconv.Itoa(idx)+".tdown")
		go parallel(client, req, files[idx], &wg)
	}
	wg.Wait()
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Concating...")
	for _, i := range files {
		tempf, _ := os.OpenFile(i, os.O_RDONLY, 0777)
		io.Copy(f, tempf)
		tempf.Close()
	}
	f.Close()
	os.RemoveAll(tempdir)
}

func parallel(client *http.Client, request *http.Request, path string, wg *sync.WaitGroup) {
	resp, err := client.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != 206 {
		log.Fatal("Unsupport Parallel")
	}
	defer resp.Body.Close()
	f, _ := os.Create(path)
	io.Copy(f, resp.Body)
	f.Close()
	wg.Done()
}
