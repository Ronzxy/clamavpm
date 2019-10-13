package main

import (
	"github.com/allegro/bigcache"
	"github.com/skygangsta/go-helper"
	"github.com/skygangsta/go-logger"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Http struct {
	cache *bigcache.BigCache
}

func NewHttp(cache *bigcache.BigCache) *Http {
	return &Http{
		cache: cache,
	}
}

// Conforms to the http.Handler interface.
func (this *Http) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	fileName := r.URL.Path[1:]
	index := strings.LastIndex(fileName, ".")

	if fileName == "" {
		w.WriteHeader(200)
		w.Write([]byte("This is a clamav private mirror, please visit https://github.com/beacon-server/clamavpm"))
		return
	}

	entry, err := this.cache.Get(fileName)
	if err != nil {
		logger.Errorf("Cannot found cached file: %s", fileName)
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}

	logger.Infof("Get cached file: %s", fileName)

	if helper.String.IsNotEqual(fileName[index:], ".cvd") {
		w.WriteHeader(200)
		w.Write(entry)
		return
	}

	var errs []error
	avDefs := ParseCVD(entry, &errs)
	if len(errs) > 0 {
		for err := range errs {
			logger.Errorf("Parsing error: %v", err)
		}

		return
	}

	timer := helper.NewTimeHelper()
	var (
		modifiedTime time.Time
	)

	modifiedTimeStr := r.Header.Get("If-Modified-Since")
	if modifiedTimeStr != "" {
		modifiedTime, err = timer.Parse("w, dd J yyyy HH:MM:SS zz", modifiedTimeStr)
		if err != nil {
			logger.Errorf("%s - If-Modified-Since Time format error", fileName)

			http.Error(w, "If-Modified-Since time format error, default: Thu, 10 Oct 2019 11:01:41 +0800\r\n", http.StatusBadRequest)
			return
		}

		modifiedTime, _ = timer.ConvertLocation(modifiedTime, "Asia/Shanghai")
		avDefs.Header.CreationTime, _ = timer.ConvertLocation(avDefs.Header.CreationTime, "Asia/Shanghai")

		logger.Tracef("%s - If-Modified-Since: %v", fileName, modifiedTime)
		logger.Tracef("%s - CVD-Creation-Time: %v", fileName, avDefs.Header.CreationTime)
	}

	w.Header().Set("Last-Modified-Time", timer.DefaultFormat(avDefs.Header.CreationTime))

	if modifiedTime.Unix() > avDefs.Header.CreationTime.Unix() {
		http.Error(w, "File Not Modified", http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(entry)))
	w.Header().Set("Content-Type", "text/octet-stream")

	w.WriteHeader(http.StatusOK)
	w.Write(entry)
}
