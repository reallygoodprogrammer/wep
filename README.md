# wep

scrape responses using [goquery](https://github.com/PuerkitoBio/goquery)
and optionally [playwright](https://github.com/playwright-community/playwright-go).
Also has the ability to spider through urls in page based on separate matching css 
selector and attribute options.

install with : `go install github.com/reallygoodprogrammer/wep@latest`

## usage:

```
Usage of wep: wep [OPTIONS] <CSS SELECTOR>
-u, --url <URL>                 set site url for initial request
-a, --attr <ATTR>               extract from ATTR attribute instead of HTML
-H, --display-url               display the url of the page with each match

-n, --headless                  run the program in chromium headless mode
-p, --playwright                use playwright instead of net/http lib for requests
-c, --concurrency <LEVEL>       set concurrency level for requests (def=1)
-t, --timeout <LEVEL>           set timeout for requests in sec (def=10)
-b, --cookie <COOKIE>           set 'Cookie' header for each request
-i, --inner                     display only the inner content of matching element

-l, --local <FILENAME>          search through local HTML file instead
-s, --stdin-urls                read urls from stdin instead of html data

-T, --traverse <CSS SELECTOR>   find new urls to spider by matching css selector
-A, --traverse-attr <ATTR>      find spider urls in ATTR of matching -T arg
-L, --leave-domain              allow spidering urls outside original domain
```
