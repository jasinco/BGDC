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

	"github.com/schollz/progressbar/v3"
)

func DownloadHandle(url string, path string) {
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		if ok, resp := CheckParallel(url); !ok {
			NormalDownload(url, path, resp)
		} else {
			PrepareParallel(url, path, resp)
		}
	} else {
		log.Fatalln("Unsupport Protocal", url)
	}
}

func CheckParallel(url string) (bool, http.Response) {
	if resp, err := http.Head(url); err == nil && resp.Header.Get("Accept-Ranges") == "bytes" && resp.ContentLength != -1 {
		return true, *resp
	} else if err != nil {
		log.Fatalln(err)
	} else {
		return false, *resp
	}
	return false, http.Response{}
}

func validateDestination(destination string) bool {
	if inf, err := os.Stat(destination); err == nil && inf.IsDir() {
		return true
	} else {
		return false
	}
}

func bytesSeperator(process int, length int) []string {

	bytesSeperate := make([]string, process)

	// use Arithmetic progression to caculate the postion of requests' byte
	// Information https://en.wikipedia.org/wiki/Arithmetic_progression
	d := int(math.Floor(float64(length) / float64(process)))
	for i := 0; i < process; i++ {
		start := 0 + d*i
		finish := d - 1 + i*d
		if i != process-1 {
			bytesSeperate[i] = fmt.Sprintf("%d-%d", start, finish)
		} else {
			bytesSeperate[i] = fmt.Sprintf("%d-", start)
		}
	}
	return bytesSeperate
}

func NormalDownload(url string, path string, head http.Response) {
	resp, err := http.Get(url)

	if err == nil && resp.StatusCode == 200 {
		var destination string

		//Path Solving
		if path == "" {
			if name := head.Header.Get("Content-Disposition"); name != "" { // Support by website, HTTP Header
				destination = strings.Split(name, "filename=")[1]
			} else {
				destination = head.Request.URL.Path[strings.LastIndex(head.Request.URL.Path, "/")+1:]
			}
		} else {
			if validateDestination(path) {
				if name := head.Header.Get("Content-Disposition"); name != "" {
					destination = filepath.Join(destination, strings.Split(name, "filename=")[1])
				} else {
					destination = filepath.Join(destination, head.Request.URL.Path[strings.LastIndex(head.Request.URL.Path, "/")+1:])
				}
			} else if filepath.Ext(path) == "" {
				os.Mkdir(path, 0777)
			} else {
				destination = path
			}
		}

		fmt.Println("Save to", destination)

		f, _ := os.Create(destination)
		defer f.Close()

		bar := progressbar.DefaultBytes(resp.ContentLength, "Download")

		io.Copy(io.MultiWriter(f, bar), resp.Body)

	} else {
		log.Fatal("HTTP", resp.StatusCode, err)
	}
	resp.Body.Close()
}

type parallelReturnObj struct {
	file string
	id   int
}

func PrepareParallel(url string, path string, head http.Response) {
	sep := bytesSeperator(6, int(head.ContentLength))

	var destination string

	if path == "" {
		if name := head.Header.Get("Content-Disposition"); name != "" {
			destination = strings.Split(name, "filename=")[1]
		} else {
			destination = head.Request.URL.Path[strings.LastIndex(head.Request.URL.Path, "/")+1:]
		}
	} else {
		if validateDestination(path) {
			if name := head.Header.Get("Content-Disposition"); name != "" {
				destination = filepath.Join(destination, strings.Split(name, "filename=")[1])
			} else {
				destination = filepath.Join(destination, head.Request.URL.Path[strings.LastIndex(head.Request.URL.Path, "/")+1:])
			}
		} else if filepath.Ext(path) == "" {
			os.Mkdir(path, 0777)
		} else {
			destination = path
		}
	}
	fmt.Println("Save to", destination)

	parallelReturn := make(chan parallelReturnObj)

	bar := progressbar.DefaultBytes(head.ContentLength, "Download")

	for idx, i := range sep { // Parallel
		go parallelDownload(url, i, idx, bar, parallelReturn)
	}

	prReturn := make([]string, 6) // Get return from other progress
	for i := 0; i < 6; i++ {
		probj := <-parallelReturn
		prReturn[probj.id] = probj.file
	}

	file, err := os.Create(destination) // Create file
	if err != nil {
		log.Fatal(err)
	}

	for _, i := range prReturn {
		tempf, _ := os.OpenFile(i, os.O_RDONLY, 0777) // Open temp file
		io.Copy(file, tempf)
		tempf.Close()
		os.Remove(i)
	}

}

func parallelDownload(url string, bytesRange string, id int, bar *progressbar.ProgressBar, parallelReturn chan parallelReturnObj) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Range", "bytes="+bytesRange)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != 206 { // bytes support, Check MDN doc
		log.Fatal("Does not support")
	}

	prReturn := new(parallelReturnObj)

	f, err := os.CreateTemp(os.TempDir(), "BGD-"+strconv.Itoa(id)+"-*.dtmp")
	if err != nil {
		log.Fatal(err)
	}
	prReturn.file = f.Name()
	prReturn.id = id

	io.Copy(io.MultiWriter(f, bar), resp.Body)

	f.Close()
	resp.Body.Close()
	parallelReturn <- *prReturn

}
