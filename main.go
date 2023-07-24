package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gocolly/colly/v2"
	"github.com/qiniu/qmgo"
	"go.mongodb.org/mongo-driver/bson"
)

type DataNews struct {
	Title    string `bson:"title"`
	Author   string `bson:"author"`
	Date     string `bson:"date"`
	Content  string `bson:"content"`
	ImageURL string `bson:"image_url"`
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

func downloadImage(URL, fileName string) error {
	// Get the response bytes from the URL
	response, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return errors.New("Received non-200 response code")
	}

	// Create the "Images" folder if it does not exist
	folderPath := "Images"
	err = os.MkdirAll(folderPath, os.ModePerm)
	if err != nil {
		return err
	}

	// Create the file in the "Images" folder
	filePath := filepath.Join(folderPath, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the bytes to the file
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

	c := colly.NewCollector()
	DetailCollector := c.Clone()

	newsURLS := []string{}
	counter := 1
	i := 1

	// Find and visit the page containing the parent div
	c.OnHTML("div.width-100.mb24.terkini", func(e *colly.HTMLElement) {
		// Find all child links
		e.ForEach("div.width-100.mb24.sm-pl15.sm-pr15", func(_ int, link *colly.HTMLElement) {
			newsURL := link.ChildAttr("a", "href")
			newsURLS = append(newsURLS, newsURL)
		})

		DetailCollector.OnHTML("div.left-section", func(d *colly.HTMLElement) {
			data := DataNews{
				Title:    d.ChildText("div.detail-title"),
				Author:   d.ChildText("div.detail-nama-redaksi"),
				Date:     d.ChildText("div.detail-date-artikel"),
				Content:  d.ChildText("div.detail-desc"),
				ImageURL: d.ChildAttr("div.detail-img img", "data-src"),
			}
			// Insert data to MongoDB
			_, err := coll.InsertOne(context.TODO(), bson.M{
				"title":   data.Title,
				"author":  data.Author,
				"date":    data.Date,
				"content": data.Content,
			})
			if err != nil {
				log.Panic("Failed Insert Data")
			}
			//Download Image File
			ImageTitle := "Image-" + strconv.Itoa(i) + ".png"
			downloadImage(data.ImageURL, ImageTitle)
			i++
		})

		for _, v := range newsURLS {
			if counter <= maxPost {
				DetailCollector.Visit(v)
				counter++
				if counter > maxPost {
					break
				}
			}
		}

	})

	err = c.Visit(URLMaker(maxPaging))
	if err != nil {
		fmt.Println("Error:", err)
	}
	fmt.Println("Done!")
}

func main() {
	var maxPost, maxPaging int

	fmt.Print("Input maxPost : ")
	fmt.Scanln(&maxPost)

	fmt.Print("Input maxPaging : ")
	fmt.Scanln(&maxPaging)

	scraperNews(maxPost, maxPaging)
}
