# wep

scrape responses using [goquery](https://github.com/PuerkitoBio/goquery)
and [playwright](https://github.com/playwright-community/playwright-go).
Works by going to url, waiting for network to idle, then extracting css-selector
query content from page. Has the ability to recurse through urls in page
based on separate matching css selector and attribute options.

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

## usage:

```
Usage of wep: wep [OPTIONS] <CSS SELECTOR>
-u, --url <URL>			set site url for initial request
-a, --attr <ATTR>		extract from ATTR attribute instead of HTML
-H, --display-url		display the url of the page with each match

-n, --headless			run the program in browserless mode when dynamic
-c, --concurrency <LEVEL>	set concurrency level for requests (def=3)
-t, --timeout <LEVEL>		set timeout for requests in sec (def=10)
-b, --cookie <COOKIE>		set 'Cookie' header for each request

-l, --local <FILENAME>		search through local HTML file instead
-s, --stdin			read HTML data from stdin instead of urls

-T, --traverse <CSS SELECTOR>	find new urls through matching css selector
-A, --traverse-attr <ATTR>	find traverse url in ATTR of match
-L, --leave-domain		allow finding urls from different domains
```
