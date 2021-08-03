package scrapper

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	ccsv "github.com/tsak/concurrent-csv-writer"
)

type extractedJob struct {
	link     string
	title    string
	location string
	salary   string
	summary  string
}

// Scrape Indeed by a term
func Scrape(term string) {
	baseURL := "https://kr.indeed.com/jobs?q=" + term + "&limit=50"
	var jobs []extractedJob
	c := make(chan []extractedJob)
	startTime := time.Now()

	totalPages := getPages(baseURL)
	for i := 0; i < totalPages; i++ {
		go getPage(baseURL, i, c)
	}

	for i := 0; i < totalPages; i++ {
		extractJobs := <-c
		jobs = append(jobs, extractJobs...)
	}
	writeJobs(jobs)
	fmt.Println("Done, extracted:", len(jobs))

	endTime := time.Now()
	fmt.Println("Operating Time: ", endTime.Sub(startTime))
}

func writeJobs(jobs []extractedJob) {
	c := make(chan []string)
	w, err := ccsv.NewCsvWriter("jobs.csv")
	checkErr(err)
	defer w.Close()

	headers := []string{"Link", "Title", "Location", "Salary", "Summary"}
	wErr := w.Write(headers)
	checkErr(wErr)

	for _, job := range jobs {
		go func(j extractedJob, c chan []string) {
			jobSlice := []string{j.link, j.title, j.location, j.salary, j.summary}
			c <- jobSlice
		}(job, c)
	}

	for i := 0; i < len(jobs); i++ {
		jobData := <-c
		jwErr := w.Write(jobData)
		checkErr(jwErr)
	}
}

func getPage(baseURL string, page int, mainC chan<- []extractedJob) {
	var jobs []extractedJob
	c := make(chan extractedJob)

	pageURL := baseURL + "&start=" + strconv.Itoa(page*50)
	fmt.Println("Requesting", pageURL)
	res, err := http.Get(pageURL)
	checkErr(err)

	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	checkErr(err)

	searchCards := doc.Find(".tapItem")
	searchCards.Each(func(i int, card *goquery.Selection) {
		go extractJob(card, c)
	})

	for i := 0; i < searchCards.Length(); i++ {
		job := <-c
		jobs = append(jobs, job)
	}
	mainC <- jobs
}

func extractJob(card *goquery.Selection, c chan<- extractedJob) {
	href, _ := card.Attr("href")
	link := getLink(href)
	title := CleanString(card.Find(".jobTitle>span").Text())
	location := CleanString(card.Find(".companyLocation").Text())
	salary := CleanString(card.Find(".salary-snippet").Text())
	summary := CleanString(card.Find(".job-snippet").Text())
	c <- extractedJob{
		link:     link,
		title:    title,
		location: location,
		salary:   salary,
		summary:  summary,
	}
}

func getLink(href string) string {
	link := "Couldn't extract link"
	str := strings.Split(href, "?jk=")
	if len(str) >= 2 {
		id := strings.Split(str[1], "&fccid=")[0]
		link = "https://kr.indeed.com/viewjob?jk=" + id
	}
	return link
}

func getPages(baseURL string) int {
	pages := 0
	res, err := http.Get(baseURL)
	checkErr(err)
	checkCode(res)

	defer res.Body.Close()

	doc, err := goquery.NewDocumentFromReader(res.Body)
	checkErr(err)

	doc.Find(".pagination").Each(func(i int, s *goquery.Selection) {
		pages = s.Find("a").Length()
	})

	return pages
}

func checkErr(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func checkCode(res *http.Response) {
	if res.StatusCode != 200 {
		log.Fatalln("Request failed with Status :", res.StatusCode)
	}
}

// CleanString cleans a string
func CleanString(str string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(str)), " ")
}
