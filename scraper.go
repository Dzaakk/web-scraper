package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gocolly/colly"
	"github.com/qiniu/qmgo"
	"gopkg.in/mgo.v2/bson"
)

type DataNews struct {
	Title   string `bson:"title"`
	Author  string `bson:"author"`
	Date    string `bson:"date"`
	Content string `bson:"content"`
}

func Post(maxPost int) int {
	count := maxPost
	if count < 0 {
		count = 1
	}
	if count > 20 {
		count = 20
	}

	return count
}

// Pagination
func URLMaker(maxPaging int) string {
	page := maxPaging
	var url = "https://nasional.sindonews.com/more/5"

	if page <= 5 {
		return url
	} else {
		number := (page + 19) / 20 * 20
		if number <= 180 {
			page = number
		} else {
			page = 180
		}
	}

	link := strconv.Itoa(page)
	url = url + "/" + link
	return url
}

func downloadFile(URL, fileName string) error {
	//Get the response bytes from the url
	response, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return errors.New("Received non 200 response code")
	}
	//Create a empty file
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	//Write the bytes to the file
	_, err = io.Copy(file, response.Body)
	if err != nil {
		return err
	}

	return nil
}

func scraperNews(maxPost, maxPaging int) {
	ctx := context.Background()
	client, err := qmgo.NewClient(ctx, &qmgo.Config{
		Uri: "mongodb://127.0.0.1:27017",
	})
	if err != nil {
		log.Fatal(err, "Please reconnect to database!")
	}
	defer func() {
		if err := client.Close(ctx); err != nil {
			panic(err)
		}
	}()

	coll := client.Database("scraper_app").Collection("news")

	News := colly.NewCollector()
	DetailNews := News.Clone()

	// Use wait group to synchronize insertions
	var wg sync.WaitGroup

	News.OnHTML("div.grid-kanal-small.height-120px.sm-height-16-9 a", func(e *colly.HTMLElement) {
		wg.Add(1) // Increment the wait group counter

		// Wait group's Done() should be called at the end of the function
		defer wg.Done()

		DetailNewsURL := e.Attr("href")
		DetailNews.OnHTML("div.left-section", func(d *colly.HTMLElement) {
			data := DataNews{
				Title:   d.ChildText("div.detail-title"),
				Author:  d.ChildText("div.detail-nama-redaksi"),
				Date:    d.ChildText("div.detail-date-artikel"),
				Content: d.ChildText("div.detail-desc"),
			}

			fmt.Printf("Scraped: %s\n", data.Title)

			// Insert data to MongoDB
			_, err := coll.InsertOne(context.TODO(), bson.M{
				"title":   data.Title,
				"author":  data.Author,
				"date":    data.Date,
				"content": data.Content,
			})
			if err != nil {
				log.Println("Failed to insert data:", err)
			}
		})

		err := DetailNews.Visit(DetailNewsURL)
		if err != nil {
			fmt.Println("Error:", err)
		}
	})

	err = News.Visit(URLMaker(maxPaging))
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Wait for all insertions to complete before proceeding
	time.Sleep(1 * time.Second)
	wg.Wait()

}

func main() {
	scraperNews(5, 5)
}
