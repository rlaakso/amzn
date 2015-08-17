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
import "strings"

import "time"
import "sort"

import "os"

import "encoding/base64"

import "crypto/hmac"
import "crypto/sha256"

import "net/http"
import "net/url"

import "launchpad.net/xmlpath"

// DELIM output delimiter
const DELIM = "\t"

// ItemAttributes Amazon webstore book attributes
type ItemAttributes struct {
	author          []string
	binding         string
	ean             string
	edition         string
	isbn            string
	pages           string
	publicationDate string
	publisher       string
	title           string
	price           string
	priceCurrency   string
}

type AWSCredentials struct {
	host      string
	accessKey string
	secret    string
}

type AWSQuery struct {
	host      string
	accessKey string
	params    map[string]string
}

// signHmacSha256 Sign AWS request using HMAC-SHA256
func signHmacSha256(data string, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	hashVal := h.Sum(nil)
	return url.QueryEscape(base64.StdEncoding.EncodeToString(hashVal))
}

// newAWSQuery constructs a new AWS Product Advertising API query
func newAWSQuery(host string, accessKey string) AWSQuery {
	var q AWSQuery
	q.host = host
	q.accessKey = accessKey
	q.params = map[string]string{
		"Service":        "AWSECommerceService",
		"Version":        "2011-08-01",
		"AssociateTag":   "PutYourAssociateTagHere",
		"Timestamp":      url.QueryEscape(time.Now().UTC().Format(time.RFC3339)),
		"AWSAccessKeyId": accessKey,
	}
	return q
}

// signRequest signs an AWS request
func signRequest(aws AWSQuery, cred AWSCredentials) (string, string) {
	// sort params by name
	var query []string
	for k, v := range aws.params {
		query = append(query, k+"="+v)
	}
	sort.Strings(query)

	// create string to be signed
	signString := fmt.Sprintf("%s\n%s\n%s\n%s", "GET", cred.host, "/onca/xml", strings.Join(query, "&"))

	// sign string
	signature := signHmacSha256(signString, cred.secret)
	return strings.Join(query, "&"), signature
}

// lookupItem from Amazon store using itemId. Uses ItemLookup method from Product Advertising API.
func lookupItem(cred AWSCredentials, itemId string) *xmlpath.Node {

	// create request
	q := newAWSQuery(cred.host, cred.accessKey)
	q.params["Operation"] = "ItemLookup"
	q.params["ItemId"] = itemId
	q.params["ResponseGroup"] = "ItemAttributes"

	// create signature
	query, signature := signRequest(q, cred)

	// make request url
	request := "http://" + cred.host + "/onca/xml" + "?" + query + "&Signature=" + signature

	// HTTP GET
	resp, _ := http.Get(request)

	// parse response xml
	root, err := xmlpath.Parse(resp.Body)
	if err != nil {
		panic(err)
	}

	return root
}

// parseItemAttributes from Amazon API response
func parseItemAttributes(node *xmlpath.Node) ItemAttributes {
	var item ItemAttributes
	auths := xmlpath.MustCompile("//Author").Iter(node)
	for auths.Next() {
		item.author = append(item.author, auths.Node().String())
	}
	item.binding, _ = xmlpath.MustCompile("//Binding").String(node)
	item.ean, _ = xmlpath.MustCompile("//EAN").String(node)
	item.edition, _ = xmlpath.MustCompile("//Edition").String(node)
	item.isbn, _ = xmlpath.MustCompile("//ISBN").String(node)
	item.pages, _ = xmlpath.MustCompile("//NumberOfPages").String(node)
	item.publicationDate, _ = xmlpath.MustCompile("//PublicationDate").String(node)
	item.publisher, _ = xmlpath.MustCompile("//Publisher").String(node)
	item.title, _ = xmlpath.MustCompile("//Title").String(node)
	item.price, _ = xmlpath.MustCompile("//ListPrice/Amount").String(node)
	item.priceCurrency, _ = xmlpath.MustCompile("//ListPrice/CurrencyCode").String(node)
	return item
}

func main() {

	if os.Getenv("AWS_KEY") == "" || len(os.Args) != 2 {
		fmt.Fprint(os.Stderr, "Usage: amzn-item-lookup <itemId>.\nAWS credentials are read from the environment variables AWS_KEY and AWS_SECRET.\n")
		os.Exit(-1)
	}

	// Construct AWS credentials
	var cred AWSCredentials
	cred.host = "ecs.amazonaws.co.uk"
	cred.accessKey = os.Getenv("AWS_KEY")
	cred.secret = os.Getenv("AWS_SECRET")

	// lookup item by id
	itemId := os.Args[1]
	itemXml := lookupItem(cred, itemId)

	// find ItemAttributes block
	xml := xmlpath.MustCompile("//ItemAttributes")
	iter := xml.Iter(itemXml)
	if !iter.Next() {
		panic("Cannot parse response! [Wrong credentials?]")
	}
	item := parseItemAttributes(iter.Node())
	//	fmt.Println(item)

	// print output
	fmt.Print(strings.Join(item.author, ", "), DELIM)
	fmt.Print(item.title, DELIM, item.publisher, DELIM, item.edition, " ed", DELIM, item.publicationDate, DELIM)
	fmt.Print(item.binding, DELIM, item.pages, " pages", DELIM, item.isbn, DELIM, item.ean, DELIM, item.price, DELIM, item.priceCurrency)
	fmt.Println()
}
