/**
Copyright (c) 2015, Risto Laakso <risto.laakso@iki.fi>

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
**/
package main

import "fmt"
import "net/http"
import "golang.org/x/net/html"
import "regexp"
import "strings"
import "bytes"
import "os"

// DELIM delimiter to be used for CSV file output
const DELIM = "\t"

// WishlistItem struct to hold item data
type WishlistItem struct {
	amazonId, author, binding, title, imageUrl, currency, price string
}

// getPage gets a webpage using HTTP
func getPage(url string) string {

	resp, _ := http.Get(url)
	doc, err := html.Parse(resp.Body)
	if err != nil {
		panic(err)
	}

	var b bytes.Buffer
	html.Render(&b, doc)
	str := b.String()

	return str
}

// parseItemData gets item data for a single item.
// page is the full html for the page.
// itemid is html id for current item.
// item is substring around current item.
//
// This function is called once for each itemName div in the page.
func parseItemData(page string, itemid string, item string) WishlistItem {

	var ret WishlistItem

	//	fmt.Println(item)

	// Amazon Item ID
	r := regexp.MustCompile("href=\"/dp/(.+)/ref") // get item id from link "/dp/ITEM_ID/ref.."
	awsnum := r.FindStringSubmatch(item)
	if len(awsnum) != 0 {
		ret.amazonId = awsnum[1]
	}

	// Author and binding
	r = regexp.MustCompile("</h5>\\s*by (.*)\\n") // author is in "by John Smith (Paperback)"
	author := r.FindStringSubmatch(item)
	if len(author) != 0 {
		r = regexp.MustCompile("(.*?)\\s\\((.*?)\\)\\s*") // split to "John Smith" and "Paperback"
		authAndBind := r.FindStringSubmatch(author[1])
		if len(authAndBind) == 0 {
			ret.author = author[1]
		} else {
			ret.author = authAndBind[1]
			ret.binding = authAndBind[2]
		}
	}

	// Title
	r = regexp.MustCompile("title=\"(.*)\" href") // title is an attribute in the itemName tag
	title := r.FindStringSubmatch(item)
	if len(title) != 0 {
		ret.title = title[1]
	}

	// Image url
	r = regexp.MustCompile("<div id=\"itemImage_" + itemid + "\"") // image url is in different tag, itemImage
	imageDivIdx := r.FindStringIndex(page)
	if len(imageDivIdx) != 0 {
		idx := imageDivIdx[1]
		r = regexp.MustCompile("<img .*? src=\"(.*?)\"") // get img tag source
		imgUrl := r.FindStringSubmatch(page[idx : idx+1000])
		if len(imgUrl) != 0 {
			ret.imageUrl = imgUrl[1]
		}
	}

	// Price
	r = regexp.MustCompile("<span id=\"itemPrice_" + itemid + "\"") // price is in separate tag, itemPrice
	imagePriceIdx := r.FindStringIndex(page)
	if len(imagePriceIdx) != 0 {
		idx := imagePriceIdx[1]
		r = regexp.MustCompile("<span .*?>\\s*(.*?)\\s*</span>") // get span content
		price := r.FindStringSubmatch(page[idx-50 : idx+150])
		if len(price) != 0 {
			// convert pound sign U+00A3 to "GBP XXX.XX", if present  // TODO check other currencies
			if len(price[1]) > 0 && price[1][0] == 0xA3 {
				ret.currency = "GBP"
				ret.price = price[1][1:]
			} else {
				ret.price = price[1]
			}
		}
	}

	return ret
}

func filter(x string) string {
	return strings.Replace(html.UnescapeString(x), "\u200B", "", -1)
}

func main() {

	// Parse command line arguments
	if len(os.Args) != 2 {
		fmt.Println("Usage: aws-wishlist-export <wishlist-id>\nWishlist ID can be found in the URL, eg http://www.amazon.co.uk/gp/registry/wishlist/THIS_IS_THE_ID/ref=..?\n")
		os.Exit(-1)
	}

	wishlistId := os.Args[1]

	// Construct wishlist URL
	host := "www.amazon.co.uk" // TODO add command line option "-country uk" or us, fr, de, ..

	// loop over all pages in the wishlist
	for pageNo := 1; ; pageNo++ {

		// get wishlist page
		pageUrl := fmt.Sprintf("http://%s/gp/registry/wishlist/%s/?page=%d", host, wishlistId, pageNo)
		page := getPage(pageUrl)

		// find all items on current page
		r := regexp.MustCompile("<a id=\"itemName_([A-Z0-9]+)\"")
		idx := r.FindAllStringIndex(page, -1)
		for _, y := range idx {

			// substring around match
			item := page[y[0] : y[1]+1000]

			// find item html id in page
			itemid := r.FindStringSubmatch(item)

			// parse item data
			wi := parseItemData(page, itemid[1], item)

			fmt.Println(
				wi.amazonId, DELIM,
				filter(wi.author), DELIM,
				filter(wi.title), DELIM,
				wi.binding, DELIM,
				wi.currency, DELIM,
				wi.price, DELIM,
				wi.imageUrl)

			//			os.Exit(0) // debug
		}

		// check if there is a next page
		r = regexp.MustCompile("<a href=\"(.*)\">\\s*Next")
		match := r.FindStringSubmatch(page)
		if len(match) == 0 {
			break // no more pages..
		}
	}
}
