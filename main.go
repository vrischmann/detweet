package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func extractTweetJSFile(path string) ([]byte, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	for _, file := range zr.File {
		if file.Name != "tweet.js" {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return nil, err
		}

		return ioutil.ReadAll(rc)
	}

	return nil, fmt.Errorf("no tweet.js file in %s", path)
}

type jsontime struct {
	inner time.Time
}

func (t *jsontime) UnmarshalJSON(data []byte) error {
	tmp, err := time.Parse(time.RubyDate, string(data[1:len(data)-1]))
	if err != nil {
		return err
	}

	t.inner = tmp

	return nil
}

func main() {
	var (
		flConsumerKey    = flag.String("consumer-key", os.Getenv("CONSUMER_KEY"), "Consumer key")
		flConsumerSecret = flag.String("consumer-secret", os.Getenv("CONSUMER_SECRET"), "Consumer secret")
		flAccessToken    = flag.String("access-token", os.Getenv("ACCESS_TOKEN"), "Access token")
		flAccessSecret   = flag.String("access-secret", os.Getenv("ACCESS_SECRET"), "Access secret")
		flRetention      = flag.String("retention", "", "Retention you want (in hour format, eg 48h)")
	)
	flag.Parse()

	if flag.NArg() < 1 {
		logrus.Fatal("need the archive path")
	}
	if *flRetention == "" {
		logrus.Fatal("need the retention")
	}

	retention, err := time.ParseDuration(*flRetention)
	if err != nil {
		logrus.Fatalf("invalid retention: %v", err)
	}
	newestDate := time.Now().Add(-retention)

	archivePath := flag.Arg(0)

	config := oauth1.NewConfig(*flConsumerKey, *flConsumerSecret)
	token := oauth1.NewToken(*flAccessToken, *flAccessSecret)
	httpClient := config.Client(oauth1.NoContext, token)

	client := twitter.NewClient(httpClient)
	_ = client

	// Get tweet ids

	tweetsData, err := extractTweetJSFile(archivePath)
	if err != nil {
		logrus.Fatal(err)
	}
	tweetsData = bytes.Replace(tweetsData, []byte(`window.YTD.tweet.part0 =`), nil, 1)

	var data []struct {
		IDStr     string   `json:"id_str"`
		CreatedAt jsontime `json:"created_at"`
	}
	if err := json.Unmarshal(tweetsData, &data); err != nil {
		logrus.Fatal(err)
	}

	// Sort by date ascending
	sort.Slice(data, func(i, j int) bool {
		return data[i].CreatedAt.inner.Before(data[j].CreatedAt.inner)
	})

	ids := make([]int64, 0, len(data))
	for _, v := range data {
		// Ignore tweets after the newest date
		if v.CreatedAt.inner.After(newestDate) {
			break
		}

		tmp, err := strconv.ParseInt(v.IDStr, 10, 64)
		if err != nil {
			logrus.Fatal(err)
		}
		ids = append(ids, tmp)
	}

	// Delete

	const perChunk = 6

	// Split in chunks of ids to limit the number of requests to twitter
	type chunk []int64
	nbChunks := len(ids) / perChunk
	chunks := make([]chunk, nbChunks)

	for i := 0; i < nbChunks; i++ {
		a, b := i*perChunk, i*perChunk+perChunk
		chunks[i] = ids[a:b]
	}

	var (
		sem = make(chan struct{}, 4)
		eg  errgroup.Group
	)

	for _, chunk := range chunks {
		for _, id := range chunk {
			id := id

			eg.Go(func() error {
				sem <- struct{}{}
				defer func() {
					<-sem
				}()

				tweet, _, err := client.Statuses.Destroy(id, nil)
				if err != nil {
					logrus.WithError(err).Warnf("unable to delete tweet %d", id)
					return nil
				}

				logrus.Printf("deleted tweet id %d, %q", id, tweet.Text)

				return nil
			})
		}

		time.Sleep(1 * time.Second)
	}

	if err := eg.Wait(); err != nil {
		logrus.Fatal(err)
	}
}
