package main

import (
	"fmt"
	"flag"
	"strings"
	"os"
	"sync"

	"github.com/playwright-community/playwright-go"
	"github.com/PuerkitoBio/goquery"
)

func main() {
	method := flag.String("m", "GET", "HTTP method to use")
	url := flag.String("u", "", "site url for request")
	con := flag.Int("c", 5, "concurrency level")
	timeout := flag.Int("t", 10, "timeout for requests")
	flag.Parse()

	query := strings.Join(flag.Args(), " ")

	pw, err := playwright.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not start playwright: %v\n", err)
		return
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
		Args:     []string{"--no-sandbox"},
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "could not start browser: %v\n", err)
		return
	}

	urls := make(chan string)
	output := make(chan string)

	var inWg sync.WaitGroup
	for i := 0; i < con; i++ {
		inWg.Add(1)
		go func() {
			defer inWg.Done()
			for url := range urls {
				page, err := browser.NewPage()
				if err != nil {
					fmt.Fprintf(os.Stderr, "could not create page: %v\n", err)
					return
				}

				resp, err := browser.Goto(url, playwright.PageGotoOptions{
					Timeout: playwright.Float(timeout),
					WaitUntil: playwright.WaitUntilStateNetworkidle,
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "could not go to page: %v\n", err)
					return
				}

				doc, err := goquery.NewDocumentFromReader(resp.Body)
				if err != nil {
					panic(err)
				}

				doc.Find(query).Each(func(i int, s *goquery.Selection) {
					html, err := s.Html()
					if err != nil {
						panic(err)
					}
					output <- html
				})

			}
		}()
	}

	var outWg sync.WaitGroup
	outWg.Add(1)
	go func() {
		defer outWg.Done()
		for out := range output {
			fmt.Printf("%s\n", out)
		}
	}()

	// created work groups, need to add url adding to urls channel, proper logic
	// for channel closing, add further options, add argument for extracting by
	// attribute possibly?
}
