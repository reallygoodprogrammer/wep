# cwep

css grep'ing responses using [goquery](https://github.com/PuerkitoBio/goquery)
and [playwright](https://github.com/playwright-community/playwright-go).
works by going to url, waiting for network to idle, then extracting css-selector
query content from page.

install with : `go install github.com/reallygoodprogrammer/cwep@latest`

## examples:

```
# extract all div elements from site.com
cwep -u "https://site.com" div

# extract all text within a-tag elements containing an href link
# that are children of span elements within div elements with class 
# 'content'
cwep -u "https://site.com" "div.content > span > a[href]"

# extract all inner content from h1 elements with class 'title'
# and p elements with class 'post' from urls in urls.txt file
cwep "h1.title, p.post" < urls.txt

# extract all src attribute values from img elements
cwep -a src img < urls.txt
```

```
Usage of cwep:
  -a string
    	extract from attribute instead of inner content
  -c int
    	concurrency level (default 3)
  -headless
        run in headless mode
  -t float
    	timeout for requests (default 10)
  -u string
    	site url for request
```
