package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	// "github.com/golang-module/carbon/v2"
)

type SSOrder struct {
	OrderID   int      `json:"id"`
	Orderskus []SSItem `json:"items"`
}

type SSItem struct {
	SKU string `json:"sku"`
	QTY int    `json:"id"`
}

func SSLoad() {

	url := "https://ssapi.shipstation.com/orders/orderId"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		fmt.Println(err)
	}

	//encoding
	data := []byte(os.Getenv("SSKEKY") + ":" + os.Getenv("SSSECRET"))
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(dst, data)
	log.Debug("Auth: ", string(dst))

	req.Header.Add("Host", "ssapi.shipstation.com")
	req.Header.Add("Authorization", string(dst))

	res, err := client.Do(req)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	log.Debug("Shipstation JSON: ", string(body))

	//Define URL strings
	// var url string
	// var link string
	// storeid := os.Getenv("BIGCOMMERCE_STOREID")
	// limit := 250

	//Define the Request URL
	//log.Debug(mindate())
	// mindate := mindate() //"2022-10-30"
	// log.Debug(mindate)
	//link = "?include_fields=sku,inventory_level,inventory_warning_level,custom_fields&inventory_level=0&limit="+strconv.Itoa(limit)+"&date_modified:min="+mindate
	// link = "?include_fields=sku,inventory_level,inventory_warning_level,mpn,brand_id&include=images&limit=" + strconv.Itoa(limit) + "&date_modified:min=" + mindate
	// url = "https://ssapi.shipstation.com"

	//Loop through the pages
	// log.Debug(ssjsonload(urlmake(url, link)))
	// log.Debug("Total Pages:", totalpages)
	// i := 0
	// for i < totalpages {
	// 	page, newlink := printProducts(jsonLoad(urlmake(url, link)))
	// 	log.Debug("Next Page Query:", page, newlink)
	// 	link = newlink
	// 	i = page
	// }
	// log.Debug("Final Data:", skulist)
	// csvmake()
}

func ssjsonload(url string) (Orders []SSOrder) {
	//Define the Request Client
	commerceClient := http.Client{
		Timeout: time.Second * 20, // Timeout after 2 seconds
	}

	//HTTP Request
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	//Setup Header
	req.Header.Set("User-Agent", "commerce-client")
	// req.Header.Add("x-auth-token", os.Getenv("SS_TOKEN"))
	req.SetBasicAuth(os.Getenv("SSKEKY"), os.Getenv("SSSECRET"))

	res, getErr := commerceClient.Do(req)
	if getErr != nil {
		log.Fatal(getErr)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}

	//unmarshall JSON
	Orders = []SSOrder{}
	jsonErr := json.Unmarshal(body, &Orders)
	if jsonErr != nil {
		log.Fatal(jsonErr)
		// log.Debug("Body:", string(body))
	}
	log.Debug("Orders:", Orders)
	return Orders
}
