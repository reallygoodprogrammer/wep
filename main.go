package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/PuerkitoBio/goquery"
	"github.com/playwright-community/playwright-go"
)

func main() {
	// arguments:
	url := flag.String("u", "", "site url for request")
	con := flag.Int("c", 3, "concurrency level")
	timeout := flag.Float64("t", 10.0, "timeout for requests")
	attr := flag.String("a", "", "extract from attribute instead of inner content")
	headless := flag.Bool("headless", false, "run in headless mode")
	local := flag.String("l", "", "read from local file path instead of making a request")
	stdinput := flag.Bool("s", false, "read html data from standard input")
	traverse_css := flag.String("T", "", "traverse urls matching css selector arg")
	traverse_attr := flag.String("A", "", "attribute to match with for '-T' css selector")
	traverse_out := flag.Bool("L", false, "traverse to urls outside domains from initial urls")
	display_url := flag.Bool("H", false, "display the page-url with each line of output")

	flag.Parse()

	query := strings.Join(flag.Args(), " ")

	// create channel for stdout
	output := make(chan string)
	var outWg sync.WaitGroup
	outWg.Add(1)
	go func() {
		defer outWg.Done()

		for out := range output {
			fmt.Printf("%s\n", out)
		}
	}()

	toOut := func(outputString string, url string) {
		if *display_url {
			output <- url + ":" + outputString
		} else {
			output <- outputString
		}
	}

	errOut := make(chan string)
	var errWg sync.WaitGroup
	errWg.Add(1)
	go func() {
		defer errWg.Done()
		for out := range errOut {
			fmt.Fprintf(os.Stderr, "%s\n", out)
		}
	}()

	toErr := func(errString string, url string) {
		if *display_url {
			errOut <- url + ":" + errString
		} else {
			errOut <- errString
		}
	}

	// create channel for new urls (when traversing)
	urls := make(chan string)
	var count int64 = 0

	// process page content
	process_content := func(content []byte, url string) {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
		if err != nil {
			toErr(fmt.Sprintf("'%v' could not make reader:\n\t%v", err), url) 
			return
		}

		doc.Find(query).Each(func(i int, s *goquery.Selection) {
			html, err := s.Html()
			if err != nil {
				toErr(fmt.Sprintf("'%v' could not get inner html:\n\t%v", url, err), url)
			} else if *attr != "" {
				val, valid := s.Attr(*attr)
				if valid {
					toOut(val, url)
				}
			} else {
				toOut(html, url)
			}
		})
	}

	// if reading from local file, just process the document
	if *local != "" {
		content, err := os.ReadFile(*local)
		if err != nil {
			toErr(fmt.Sprintf("'%s' could not read file:\n\t%v", *local, err), "local-file")
		} else {
			process_content(content, "local-file")
		}
	} else if *stdinput {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			toErr(fmt.Sprintf("could not read stdin:\n\t%v", err), "stdin")
		} else {
			func() {
				process_content(content, "stdin")
			}()
		}
	} else {
		// start playwright
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

		// make timeout in seconds
		*timeout *= 1000

		// diminish the work counter, if zero there are no more urls
		diminish := func() {
			if atomic.AddInt64(&count, -1) == 0 {
				close(urls)
			}
		}

		// declaring runit here so it can be called in traverse
		var runit func(url string)

		// urls that have already been visited
		visited := make(map[string]bool)
		var visitedLock sync.Mutex

		// domains that are allowed for traversing
		visited_domains := make(map[string]bool)
		var visitedDomainsLock sync.Mutex

		// add a domain to the visited_domains map
		addVisitedDomain := func(url string) {
			hostname := getHostname(url)

			visitedDomainsLock.Lock()
			defer visitedDomainsLock.Unlock()
			visited_domains[hostname] = true
		}

		// check if the url is in the visited map
		inVisited := func(url string) bool {
			value := func() bool {
				visitedLock.Lock()
				defer visitedLock.Unlock()

				if !visited[url] {
					visited[url] = true
					return false
				}
				return true
			}()
			if value {
				return true
			}
			
			// code below here checks if the url in the
			// allowed domains map
			if *traverse_out {
				return false
			}
			
			hostname := getHostname(url)

			visitedDomainsLock.Lock()
			defer visitedDomainsLock.Unlock()

			return !visited_domains[hostname]
		}

		// find matching traversal urls and add them to urls channel (or process them)
		traverse := func(content []byte, url string) {
			doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
			if err != nil {
				toErr(fmt.Sprintf("could not make reader for traversal:\n\t%v", err), url)
				return
			}

			doc.Find(*traverse_css).Each(func(i int, s *goquery.Selection) {
				html, err := s.Html()
				if err != nil {
					toErr(fmt.Sprintf("could not get inner html:\n\t%v", err), url)
				} else if *traverse_attr != "" {
					val, valid := s.Attr(*attr)
					if valid {
						atomic.AddInt64(&count, 1)
						traverse_val := absolute_url(url, val)
						if traverse_val != "" && !inVisited(traverse_val) {
							select {
							case urls <- traverse_val:
								// i love go
							default:
								runit(traverse_val)
							}
						}
					}
				} else {
					atomic.AddInt64(&count, 1)
					traverse_val := absolute_url(url, html)
					if traverse_val != "" && !inVisited(traverse_val) {
						select {
						case urls <- traverse_val:
							// go loves u
						default:
							runit(traverse_val)
						}
					}
				}
			})
		}

		runit = func(url string) {
			defer diminish()
			page, err := browser.NewPage()
			if err != nil {
				toErr(fmt.Sprintf("could not create page:\n\t%v", err), url)
				return
			}

			_, err = page.Goto(url, playwright.PageGotoOptions{
				Timeout: playwright.Float(*timeout),
			})
			if err != nil {
				toErr(fmt.Sprintf("could not go to page:\n\t%v", err), url)
				return
			}

			ni := playwright.LoadState("networkidle")
			err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State:   &ni,
				Timeout: timeout,
			})

			content, err := page.Content()
			if err != nil {
				toErr(fmt.Sprintf("could not get page content:\n\t%v", err), url)
				return
			}

			process_content([]byte(content), url)
			page.Close()

			traverse([]byte(content), url)
		}

		worker := func() {
			for url := range urls {
				func() {
					runit(url)
				}()
			}
		}

		var inWg sync.WaitGroup
		for i := 0; i < *con; i++ {
			inWg.Add(1)
			go func() {
				defer inWg.Done()
				worker()
			}()
		}

		if *url != "" {
			atomic.AddInt64(&count, 1)
			addVisitedDomain(*url)
			urls <- *url
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				atomic.AddInt64(&count, 1)
				addVisitedDomain(*url)
				urls <- scanner.Text()
			}
		}

		inWg.Wait()
	}

	close(output)
	outWg.Wait()
}

func absolute_url(base string, relative string) string {
	baseUrl, err := url.Parse(base)
	if err != nil {
		return ""
	}

	relativeUrl, err := url.Parse(relative)
	if err != nil {
		return ""
	}

	resolved := baseUrl.ResolveReference(relativeUrl).String()

	if resolved == base {
		return ""
	}
	return resolved
}


func getHostname(urlString string) string {
	urlObj, err := url.Parse(urlString)
	if err != nil {
		return ""
	}
	return urlObj.Hostname()
}
