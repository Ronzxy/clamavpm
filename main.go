/* Copyright 2018 sky<skygangsta@hotmail.com>. All rights reserved.
 *
 * Licensed under the Apache License, version 2.0 (the "License").
 * You may not use this work except in compliance with the License, which is
 * available at www.apache.org/licenses/LICENSE-2.0
 *
 * This software is distributed on an "AS IS" basis, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied, as more fully set forth in the License.
 *
 * See the NOTICE file distributed with this work for information regarding copyright ownership.
 */

package main

import (
	"fmt"
	"github.com/allegro/bigcache"
	"github.com/robfig/cron"
	"github.com/skygangsta/go-helper"
	"github.com/skygangsta/go-logger"
	"github.com/urfave/cli"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {

	app := cli.NewApp()

	app.Author = "skygangsta"
	app.Email = "skygangsta@hotmail.com"
	app.Version = "v19.10.13.1"
	app.Usage = "ClamAV Private Mirror"

	// Command and options
	app.Commands = []cli.Command{
		{
			Name:    "mirror",
			Aliases: []string{"m"},
			Usage:   fmt.Sprintf("Starting ClamAV private mirror"),
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "mirror",
					Value: "",
					Usage: "Append ClamAV database mirror, multiple split with ';'",
				},
				cli.IntFlag{
					Name:  "follow",
					Value: 1,
					Usage: "Follow the diff cvd file",
				},
				cli.StringFlag{
					Name:  "cron",
					Value: "0 */3 * * *",
					Usage: "Set cron to update ClamAV database",
				},
			},
			Action: func(ctx *cli.Context) {
				var (
					mirror = ctx.String("mirror")
					follow = ctx.Bool("follow")
					spec   = ctx.String("cron")
				)

				// overkill, but it's a sane library.
				// we're going to cache the AV definition files.
				cache, err := bigcache.NewBigCache(bigcache.Config{
					MaxEntrySize:       500,
					Shards:             1024,
					LifeWindow:         time.Hour * 24,
					MaxEntriesInWindow: 1000 * 10 * 60,
					Verbose:            true,
					HardMaxCacheSize:   0,
				})

				if err != nil {
					logger.Errorf("Cannot initialise cache. %s", err)
				}

				loadCVD("main.cvd", cache)
				loadCVD("daily.cvd", cache)
				loadCVD("bytecode.cvd", cache)

				var mirrors []string

				if helper.NewStringHelper().IsNotEmpty(mirror) {
					mirrors = strings.Split(mirror, ";")
					logger.Info("Append ClamAV database mirror %s", mirrors)
				}

				mirrors = append(mirrors, "https://database.clamav.net", "https://pivotal-clamav-mirror.s3.amazonaws.com")

				// let the initial seed run in the background so the web server can start.
				logger.Info("Starting initial seed in the background.")
				dl := NewCVDDownloader(mirrors, follow)

				go dl.Download(cache)

				// start a new crontab asynchronously.
				c := cron.New()
				err = c.AddFunc(spec, func() {
					logger.Debug("Start update by cron")
					NewCVDDownloader(mirrors, follow).Download(cache)
				})
				if err != nil {
					logger.Error(err)
				}
				logger.Infof("Starting cron with %s", spec)
				c.Start()

				// 设置http服务
				s := &http.Server{
					Addr:              ":8080",
					Handler:           NewHttp(cache),
					ReadHeaderTimeout: 30 * 1000 * time.Millisecond,
					ReadTimeout:       7200 * 1000 * time.Millisecond,
					WriteTimeout:      7200 * 1000 * time.Millisecond,
					MaxHeaderBytes:    1048576,
				}

				logger.Fatal(s.ListenAndServe())
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		logger.Fatal(err)
	}
}

func loadCVD(fileName string, cache *bigcache.BigCache) {
	var (
		file *os.File
		buf  []byte
		err  error
	)
	logger.Infof("Load file %s form disk", fileName)

	file, err = os.Open(fileName)
	if err != nil {
		logger.Errorf("Load file form disk error: %s", err.Error())
	} else {
		defer file.Close()

		buf, err = ioutil.ReadAll(file)
		if err != nil {
			logger.Errorf("Read file error: %s", err.Error())
		}
		cache.Set(fileName, buf)
	}
}
