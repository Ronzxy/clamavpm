package main

import (
	"fmt"
	"github.com/allegro/bigcache"
	"github.com/skygangsta/go-helper"
	"github.com/skygangsta/go-logger"
	"strings"
	"sync"
)

// CVDDownloader is the base structure for grabbing the necessary files.
type CVDDownloader struct {
	downloader *helper.HttpHelper
	Waiter     sync.WaitGroup
	Types      []string
	Mirrors    []string
	Follow     bool
}

// NewCVDDownloader will create a new download client which manages the CVD files.
func NewCVDDownloader(mirrors []string, f bool) *CVDDownloader {
	return &CVDDownloader{
		Types: []string{
			"main.cvd",
			"bytecode.cvd",
			"daily.cvd",
		},
		Mirrors:    mirrors,
		Follow:     f,
		downloader: helper.NewHttpHelper(60, false),
	}
}

// DownloadDatabase downloads the AV definitions and some other basic business logic. It uses the predefined cache to
// save files.
func (d *CVDDownloader) Download(c *bigcache.BigCache) {
	for idx := range d.Types {
		d.Waiter.Add(1)

		go d.DownloadFile(d.Types[idx], c)
	}
	d.Waiter.Wait()

	logger.Info("Done downloading definitions.")
}

// downloadFile performs the download and places it in the /tmp directory.
func (d *CVDDownloader) DownloadFile(fileName string, c *bigcache.BigCache) {
	defer d.Waiter.Done()

	var (
		body []byte
		err  error

		version = 0
		index   = strings.LastIndex(fileName, ".")
	)

	for _, mirror := range d.Mirrors {
		cvdURL := fmt.Sprintf("%s/%s", mirror, fileName)

		logger.Infof("Downloading definition %s", cvdURL)

		body, err = d.downloader.Get(cvdURL)
		if err == nil {
			break
		}

		logger.Errorf("Failed downloading definition %s with error: %s", cvdURL, err.Error())
	}

	if body != nil {
		// parse the CVD and make sure it's valid, otherwise, exit.

		if helper.String.IsEqual(fileName[index:], ".cvd") {
			var (
				errs   []error
				avDefs *ClamAV
			)

			avDefs = ParseCVD(body, &errs)

			if len(errs) > 0 {
				for err := range errs {
					logger.Errorf("Parsing error: %v", err)
				}
				return
			}

			if !avDefs.Header.MD5Valid {
				logger.Errorf("filename %s the md5 is not valid, will not add to cache.", fileName)
				return
			}

			err = helper.File.Save(fileName, body)
			if err != nil {
				logger.Infof("Write file %s error: %s", fileName, err.Error())
			}

			version = int(avDefs.Header.Version)
		}

		if err = c.Set(fileName, body); err != nil {
			logger.Errorf("cannot add %s to cache! %s", fileName, err)
		}

		logger.Infof("Add file %s to cache!", fileName)

		if d.Follow && version != 0 {
			cDiffFile := fmt.Sprintf("%s-%d.cdiff", fileName[:index], version)

			logger.Debugf("Downloading diff file %s", cDiffFile)
			d.Waiter.Add(1)
			go d.DownloadFile(cDiffFile, c)
		}
	}
}
