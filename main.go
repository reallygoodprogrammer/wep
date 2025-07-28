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

const usage = `Usage of wep: wep [OPTIONS] <CSS SELECTOR>
-u, --url <URL>			set site url for initial request
-a, --attr <ATTR>		extract from ATTR attribute instead of HTML
-H, --display-url		display the url of the page with each match

-c, --concurrency <LEVEL>	set concurrency level for requests (def=3)
-t, --timeout <LEVEL>		set timeout for requests in sec (def=10)
-n, --headless			run the program in browserless mode

-l, --local <FILENAME>		search through local HTML file instead
-s, --stdin			read HTML data from stdin instead of urls

-T, --traverse <CSS SELECTOR>	find new urls through matching css selector
-A, --traverse-attr <ATTR>	find traverse url in ATTR of match
-L, --leave-domain		allow finding urls from different domains
`

func main() {
	// arguments:
	var url string
	var con int
	var timeout float64
	var attr string
	var headless bool
	var local string
	var stdinput bool
	var traverse_css string
	var traverse_attr string
	var traverse_out bool
	var display_url bool

	flag.StringVar(&url, "u", "", "site url for request")
	flag.StringVar(&url, "url", "", "site url for request")
	flag.IntVar(&con, "c", 3, "concurrency level")
	flag.IntVar(&con, "concurrency", 3, "concurrency level")
	flag.Float64Var(&timeout, "t", 10.0, "timeout for requests")
	flag.Float64Var(&timeout, "timeout", 10.0, "timeout for requests")
	flag.StringVar(&attr, "a", "", "extract from attribute instead of inner content")
	flag.StringVar(&attr, "attr", "", "extract from attribute instead of inner content")
	flag.BoolVar(&headless, "n", false, "run in headless mode")
	flag.BoolVar(&headless, "headless", false, "run in headless mode")
	flag.StringVar(&local, "l", "", "read from local file path instead of making a request")
	flag.StringVar(&local, "local", "", "read from local file path instead of making a request")
	flag.BoolVar(&stdinput, "s", false, "read html data from standard input")
	flag.BoolVar(&stdinput, "stdin", false, "read html data from standard input")
	flag.StringVar(&traverse_css, "T", "", "traverse urls matching css selector arg")
	flag.StringVar(&traverse_css, "traverse", "", "traverse urls matching css selector arg")
	flag.StringVar(&traverse_attr, "A", "", "attribute to match with for '-T' css selector")
	flag.StringVar(&traverse_attr, "traverse-attr", "", "attribute to match with for '-T' css selector")
	flag.BoolVar(&traverse_out, "L", false, "allow traversal to outside domains")
	flag.BoolVar(&traverse_out, "leave-domain", false, "allow traversal to outside domains")
	flag.BoolVar(&display_url, "H", false, "display the page-url with each line of output")
	flag.BoolVar(&display_url, "display-url", false, "display the page-url with each line of output")

	flag.Usage = func() { fmt.Printf("%s", usage) }
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
		if display_url {
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
		if display_url {
			errOut <- url + ":" + errString
		} else {
			errOut <- errString
		}
	}

	// create channel for new urls (when traversing)
	urls := make(chan string)
	var count int64 = 0

	// process page content (find matching things)
	process_content := func(content []byte, url string) {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
		if err != nil {
			toErr(fmt.Sprintf("could not make reader: %v", err), url)
			return
		}

		doc.Find(query).Each(func(i int, s *goquery.Selection) {
			html, err := s.Html()
			if err != nil {
				toErr(fmt.Sprintf("could not get inner html: %v", url, err), url)
			} else if attr != "" {
				val, valid := s.Attr(attr)
				if valid {
					toOut(val, url)
				}
			} else {
				toOut(html, url)
			}
		})
	}

	// if reading from local file, just process the document
	if local != "" {
		content, err := os.ReadFile(local)
		if err != nil {
			toErr(fmt.Sprintf("'%s' could not read file: %v", local, err), "local-file")
		} else {
			process_content(content, "local-file")
		}
	} else if stdinput {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			toErr(fmt.Sprintf("could not read stdin: %v", err), "stdin")
		} else {
			func() {
				process_content(content, "stdin")
			}()
		}
	} else {
		pw, err := playwright.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not start playwright: %v", err)
			os.Exit(1)
		}
		defer pw.Stop()

		browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(headless),
			Args:     []string{"--no-sandbox"},
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "could not start browser: %v", err)
			os.Exit(1)
		}

		timeout *= 1000

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
			if traverse_out {
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
				toErr(fmt.Sprintf("could not make reader for traversal: %v", err), url)
				return
			}

			doc.Find(traverse_css).Each(func(i int, s *goquery.Selection) {
				html, err := s.Html()
				if err != nil {
					toErr(fmt.Sprintf("could not get inner html: %v", err), url)
				} else if traverse_attr != "" {
					val, valid := s.Attr(traverse_attr)
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

		// worker function for fetching and processing url
		runit = func(url string) {
			defer diminish()
			page, err := browser.NewPage()
			if err != nil {
				toErr(fmt.Sprintf("could not create page: %v", err), url)
				return
			}

			_, err = page.Goto(url, playwright.PageGotoOptions{
				Timeout: playwright.Float(timeout),
			})
			if err != nil {
				toErr(fmt.Sprintf("could not go to page: %v", err), url)
				return
			}

			ni := playwright.LoadState("networkidle")
			err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
				State:   &ni,
				Timeout: &timeout,
			})

			content, err := page.Content()
			if err != nil {
				toErr(fmt.Sprintf("could not get page content: %v", err), url)
				return
			}

			process_content([]byte(content), url)
			page.Close()

			traverse([]byte(content), url)
		}

		// wrapper function for input channel and calling
		// 'worker' function
		worker := func() {
			for url := range urls {
				func() {
					runit(url)
				}()
			}
		}

		// input workers
		var inWg sync.WaitGroup
		for i := 0; i < con; i++ {
			inWg.Add(1)
			go func() {
				defer inWg.Done()
				worker()
			}()
		}

		// if url argument provided
		if url != "" {
			atomic.AddInt64(&count, 1)
			addVisitedDomain(url)
			urls <- url
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				url = scanner.Text()
				atomic.AddInt64(&count, 1)
				addVisitedDomain(url)
				urls <- url
			}
		}

		inWg.Wait()
	}

	close(output)
	outWg.Wait()
}

// return absolute url based on base and relative urls
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

// return hostname of the url (for domain comparisons)
func getHostname(urlString string) string {
	urlObj, err := url.Parse(urlString)
	if err != nil {
		return ""
	}
	return urlObj.Hostname()
}
