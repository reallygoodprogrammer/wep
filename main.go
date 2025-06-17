package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
)

func main() {
	url := flag.String("u", "", "site url for request")
	con := flag.Int("c", 3, "concurrency level (default=3)")
	timeout := flag.Float64("t", 10.0, "timeout for requests (default=10)")
	attr := flag.String("a", "", "extract from attribute instead of inner content")
	headless := flag.Bool("headless", false, "run in headless mode")
	flag.Parse()

	query := strings.Join(flag.Args(), " ")

	pw, err := playwright.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not start playwright:\n\t%v\n", err)
		os.Exit(1)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(*headless),
		Args:     []string{"--no-sandbox"},
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "could not start browser:\n\t%v", err)
		os.Exit(1)
	}

	urls := make(chan string)
	output := make(chan string)
	*timeout *= 1000

	var inWg sync.WaitGroup
	for i := 0; i < *con; i++ {
		inWg.Add(1)
		go func() {
			defer inWg.Done()
			for url := range urls {
				page, err := browser.NewPage()
				if err != nil {
					output <- fmt.Sprintf("'%v' could not create page:\n\t%v", url, err)
					return
				}
				defer page.Close()

				_, err = page.Goto(url, playwright.PageGotoOptions{
					Timeout:   playwright.Float(*timeout),
					WaitUntil: playwright.WaitUntilStateNetworkidle,
				})
				if err != nil {
					output <- fmt.Sprintf("'%v' could not go to page:\n\t%v", url, err)
					return
				}

				content, err := page.Content()
				if err != nil {
					output <- fmt.Sprintf("'%v' could not get page content:\n\t%v", url, err)
					return
				}

				doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(content)))
				if err != nil {
					output <- fmt.Sprintf("'%v' could not make reader:\n\t%v", url, err)
					return
				}

				doc.Find(query).Each(func(i int, s *goquery.Selection) {
					html, err := s.Html()
					if err != nil {
						output <- fmt.Sprintf("'%v' could not get inner html:\n\t%v", url, err)
					} else if *attr != "" {
						val, valid := s.Attr(*attr)
						if valid {
							output <- val
						}
					} else {
						output <- html
					}
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

	if *url != "" {
		urls <- *url
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			urls <- scanner.Text()
		}
	}

	close(urls)
	inWg.Wait()
	close(output)
	outWg.Wait()
}
