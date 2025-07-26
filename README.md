# wep

scrape responses using [goquery](https://github.com/PuerkitoBio/goquery)
and [playwright](https://github.com/playwright-community/playwright-go).
Works by going to url, waiting for network to idle, then extracting css-selector
query content from page. Has the ability to recurse through urls in page.

install with : `go install github.com/reallygoodprogrammer/wep@latest`

## examples:

```
# extract all div elements from site.com
wep -u "https://site.com" div

# extract all text within a-tag elements containing an href link
# that are children of span elements within div elements with class 
# 'content'
wep -u "https://site.com" "div.content > span > a[href]"

# extract all inner content from h1 elements with class 'title'
# and p elements with class 'post' from urls in urls.txt file
wep "h1.title, p.post" < urls.txt

# extract all src values from img attributes and recurse to
# other pages using a elements href values with class "rec"
wep -a src -T "a.rec[href]" -A href "img[src]"

# extract all src attribute values from img elements
wep -a src img < urls.txt
```

```
Usage of wep:
  -A string
    	attribute to match with for '-T' css selector
  -T string
    	traverse urls matching css selector arg
  -a string
    	extract from attribute instead of inner content
  -c int
    	concurrency level (default 3)
  -headless
    	run in headless mode
  -l string
    	read from local file path instead of making a request
  -s	read html data from standard input
  -t float
    	timeout for requests (default 10)
  -u string
    	site url for request
```
