package main

import (
	"testing"
	"time"

	"github.com/allegro/bigcache"
)

func newTempCache() *bigcache.BigCache {
	c, _ := bigcache.NewBigCache(bigcache.DefaultConfig(time.Second * 1))
	return c
}

func TestDownloader_DownloadFile(t *testing.T) {
	testCache := newTempCache()
	testDL := NewCVDDownloader([]string{"http://10.10.21.119:8080"}, false)

	//t.Logf("testing with %s mirror", primaryMirror)
	fileName := "daily"
	testDL.Waiter.Add(1)
	testDL.DownloadFile(fileName, testCache)

	if _, err := testCache.Get("daily"); err != nil {
		t.Error(err)
		if err.Error() == "Could not retrieve entry from cache" {
			t.Log("bad download.")
			t.SkipNow()
		}
		t.Fail()
	}
}

func TestDownloader_Download(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test.")
	}
	testCache := newTempCache()
	testDL := NewCVDDownloader([]string{"http://10.10.21.119:8080"}, false)

	testDL.Download(testCache)

	var need = []string{"daily", "main", "bytecode"}
	for file := range need {
		if _, err := testCache.Get(need[file]); err != nil {
			t.Error(err)
			if err.Error() == "Could not retrieve entry from cache" {
				t.Log("bad download.")
				t.SkipNow()
			}
			t.Fail()
		}
	}
}
