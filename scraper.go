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

	"github.com/gocolly/colly/v2"
	"github.com/qiniu/qmgo"
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

	// coll := client.Database("scraper_app").Collection("news")

	News := colly.NewCollector()
	DetailNews := News.Clone()

	counter := 1
	News.OnHTML("div.width-100.mb24.terkini", func(e *colly.HTMLElement) {

		e.ForEach("div.width-100.mb24.sm-pl15.sm-pr15", func(_ int, link *colly.HTMLElement) {
			if counter <= maxPost {
				NewsURL := link.ChildAttr("a", "href")
				DetailNews.OnHTML("div.left-section", func(d *colly.HTMLElement) {
					data := DataNews{
						Title:   d.ChildText("div.detail-title"),
						Author:  d.ChildText("div.detail-nama-redaksi"),
						Date:    d.ChildText("div.detail-date-artikel"),
						Content: d.ChildText("div.detail-desc"),
					}
					fmt.Printf("News-%d\n", counter)
					fmt.Printf("Title : %s\n", data.Title)
					fmt.Printf("Author: %s\n", data.Author)
					fmt.Printf("Date: %s\n", data.Date)
					fmt.Printf("Content: %s", data.Content)
					fmt.Println("-----------------------------")
					// Insert data to MongoDB
					// _, err := coll.InsertOne(context.TODO(), bson.M{
					// 	"title":   data.Title,
					// 	"author":  data.Author,
					// 	"date":    data.Date,
					// 	"content": data.Content,
					// })
					if err != nil {
						log.Println("Failed to insert data:", err)
					}
				})
				err := DetailNews.Visit(NewsURL)
				if err != nil {
					fmt.Println("Error:", err)
				}
				counter++
			}
		})
	})

	err = News.Visit(URLMaker(maxPaging))
	if err != nil {
		fmt.Println("Error:", err)
	}

}

func main() {
	scraperNews(2, 5)
}
