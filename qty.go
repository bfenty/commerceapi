package main

import (
	// "encoding/csv"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type product struct {
	Data []struct {
		ID                    int           `json:"id"`
		Sku                   string        `json:"sku"`
		Brand_ID              int           `json:"brand_id"`
		InventoryLevel        int           `json:"inventory_level"`
		InventoryWarningLevel int           `json:"inventory_warning_level"`
		MPN                   string        `json:"mpn"`
		Detail                []customfield `json:"custom_fields"`
	} `json:"data"`
	Meta struct {
		Pagination struct {
			Total       int `json:"total"`
			Count       int `json:"count"`
			PerPage     int `json:"per_page"`
			CurrentPage int `json:"current_page"`
			TotalPages  int `json:"total_pages"`
			Links       struct {
				Next    string `json:"next"`
				Current string `json:"current"`
			} `json:"links"`
			TooMany bool `json:"too_many"`
		} `json:"pagination"`
	} `json:"meta"`
}

type customfield struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type sku struct {
	SKU       string
	Qty       int
	ID        int
	Factory   int
	SupplySKU string
}

var skulist []sku

func mindate() (val string) {
	//open connection to database
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/purchasing"
	db, err := sql.Open("mysql",
		connectstring)
	if err != nil {
		fmt.Println("Message: ", err.Error())
	}

	//Test Connection
	pingErr := db.Ping()
	if pingErr != nil {
		fmt.Println("Message: ", err.Error())
	}

	//Get start date
	fmt.Println("Getting min date...")
	var testquery string = "SELECT date_add(max(modified),INTERVAL -1 DAY) FROM `skus`"
	rows2, err := db.Query(testquery)
	if err != nil {
		fmt.Println(err.Error())
	}
	// var val string
	if rows2.Next() {
		rows2.Scan(&val)
	}
	fmt.Println("Min Date: ", val)
	date, _ := time.Parse("2006-01-02 15:04:05", val)
	fmt.Println("DATE: ", date.Format("2006-01-02"))
	return date.Format("2006-01-02")
}

func QTYUpdate(skus []sku) {

	//open connection to database
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/purchasing"
	db, err := sql.Open("mysql",
		connectstring)
	if err != nil {
		fmt.Println("Message: ", err.Error())
	}

	//Test Connection
	pingErr := db.Ping()
	if pingErr != nil {
		fmt.Println("Message: ", err.Error())
	}

	for i := range skus {
		var newquery string = "UPDATE `skus` SET `inventory_qty`=? WHERE sku_internal=?"
		rows, err := db.Query(newquery, skus[i].Qty, skus[i].SKU)
		defer rows.Close()
		if err != nil {
			fmt.Println("Message: ", err.Error())
			rows.Close()
		}
		err = rows.Err()
		if err != nil {
			fmt.Println("Message: ", err.Error())
			rows.Close()
		}
		rows.Close()
	}
}

// Creates the URL by combining the url and link
func urlmake(url string, linkvalue string) (urlfinal string) {
	value := url + linkvalue
	return value
}

// loads JSON and returns a slice
func jsonLoad(url string) (products product) {
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
	req.Header.Add("x-auth-token", os.Getenv("BIGCOMMERCE_TOKEN"))

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
	products = product{}
	jsonErr := json.Unmarshal(body, &products)
	if jsonErr != nil {
		log.Fatal(jsonErr)
		// fmt.Println("Body:", string(body))
	}
	// fmt.Println("Products:", products)
	return products
}

// prints out the products slice
func printProducts(products product) (page int, link string) {
	var tempsku sku
	for i := range products.Data {
		tempsku.SKU = products.Data[i].Sku
		tempsku.Qty = products.Data[i].InventoryLevel
		tempsku.ID = products.Data[i].ID
		tempsku.Factory = products.Data[i].Brand_ID
		tempsku.SupplySKU = products.Data[i].MPN
		skulist = append(skulist, tempsku)
		//			}
	}
	QTYUpdate(skulist)
	link = products.Meta.Pagination.Links.Next
	return products.Meta.Pagination.CurrentPage, link
}

func qty() {
	//Define URL strings
	var url string
	var link string
	storeid := os.Getenv("BIGCOMMERCE_STOREID")
	limit := 250

	//Define the Request URL
	fmt.Println(mindate())
	mindate := "2022-10-30"
	//link = "?include_fields=sku,inventory_level,inventory_warning_level,custom_fields&inventory_level=0&limit="+strconv.Itoa(limit)+"&date_modified:min="+mindate
	link = "?include_fields=sku,inventory_level,inventory_warning_level,mpn,brand_id&include=custom_fields&limit=" + strconv.Itoa(limit) + "&date_modified:min=" + mindate
	url = "https://api.bigcommerce.com/stores/" + storeid + "/v3/catalog/products"

	//Loop through the pages
	totalpages := jsonLoad(urlmake(url, link)).Meta.Pagination.TotalPages
	fmt.Println("Total Pages:", totalpages)
	i := 0
	for i < totalpages {
		page, newlink := printProducts(jsonLoad(urlmake(url, link)))
		fmt.Println("Next Page Query:", page, newlink)
		link = newlink
		i = page
	}
	// fmt.Println("Final Data:", skulist)
	// csvmake()
}
