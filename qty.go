package main

import (
	// "encoding/csv"
	"database/sql"
	"encoding/json"
	"io/ioutil"

	// "log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gosuri/uiprogress"
	log "github.com/sirupsen/logrus"
)

type product struct {
	Data []struct {
		ID                    int           `json:"id"`
		Sku                   string        `json:"sku"`
		Brand_ID              int           `json:"brand_id"`
		InventoryLevel        int           `json:"inventory_level"`
		InventoryWarningLevel int           `json:"inventory_warning_level"`
		MPN                   string        `json:"mpn"`
		Modified              string        `json:"date_modified"`
		Detail                []customfield `json:"custom_fields"`
		Images                []Image       `json:"images"`
		Price                 float64       `json:"price"`
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

type Image struct {
	URL_Standard *string `json:"url_standard"`
	URL_Thumb    *string `json:"url_thumbnail"`
	URL_Tiny     *string `json:"url_tiny"`
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
	Modified  string
	Price     float64
	Skuimage  Image
}

func mindate() (val string) {
	//open connection to database
	log.Info("Opening DB Connection")
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/purchasing"
	db, err := sql.Open("mysql",
		connectstring)
	if err != nil {
		log.Error("Error opening Database: ", err.Error())
	}

	//Test Connection
	pingErr := db.Ping()
	if pingErr != nil {
		log.Error("Error testing DB Connection: ", err.Error())
	}

	//Get start date
	log.Info("Getting min date...")
	var testquery string = "SELECT date_add(max(modified),INTERVAL -3 DAY) FROM `skus`"
	rows2, err := db.Query(testquery)
	if err != nil {
		log.Error("Error retrieving min date: ", err.Error())
	}
	// var val string
	if rows2.Next() {
		rows2.Scan(&val)
	}
	log.Info("Min Date: ", val)
	date, _ := time.Parse("2006-01-02 15:04:05", val)
	log.Info("DATE: ", date.Format("2006-01-02"))
	return date.Format("2006-01-02")
}

func QTYUpdate(skus []sku) {

	if len(skus) == 0 {
		log.Info("No SKUs in Slice")
		return
	}

	//open connection to database
	log.Info("Opening DB Connection")
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/purchasing"
	db, err := sql.Open("mysql",
		connectstring)
	if err != nil {
		log.Error("Error opening Database: ", err.Error())
	}

	//Test Connection
	pingErr := db.Ping()
	if pingErr != nil {
		log.Error("Error testing DB Connection: ", err.Error())
	}

	log.Info("Updating Quantity for ", len(skus), " SKUs")

	// Initialize progress bar
	log.Info("Updating Order QTY in Database")
	uiprogress.Start()                                                     // start rendering
	bar := uiprogress.AddBar(len(skus)).AppendCompleted().PrependElapsed() // add a new bar
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return "Processing: " // prepend the current processing state
	})
	for i := range skus {
		bar.Incr() //update progress bar

		var newquery string = "UPDATE `skus` SET `inventory_qty`=?,price=?,url_thumb=?,url_standard=?,url_tiny=? WHERE sku_internal=REPLACE(?,' ','')"
		rows, err := db.Query(newquery, skus[i].Qty, skus[i].Price, skus[i].Skuimage.URL_Thumb, skus[i].Skuimage.URL_Standard, skus[i].Skuimage.URL_Tiny, skus[i].SKU)

		defer rows.Close()
		if err != nil {

			log.Error("Error Updating Qty: ", err.Error())
			rows.Close()
		}
		err = rows.Err()
		if err != nil {
			log.Error("Error Updating Qty: ", err.Error())
			rows.Close()
		}
		rows.Close()

		// log.Debug("Modified Date: ", skus[i].Modified)
		newquery = "INSERT INTO qty (sku_internal,inventory_qty,restock_qty,modified,restock_date) VALUES(REPLACE(?,' ',''),?,?,CURRENT_TIMESTAMP(),CURRENT_TIMESTAMP()) ON DUPLICATE KEY UPDATE inventory_qty=?,restock_date=IF(?>restock_qty,CURRENT_TIMESTAMP(),restock_date),restock_qty=IF(?>restock_qty,?,restock_qty),modified=CURRENT_TIMESTAMP()"
		rows, err = db.Query(newquery, skus[i].SKU, skus[i].Qty, skus[i].Qty, skus[i].Qty, skus[i].Qty, skus[i].Qty, skus[i].Qty)

		defer rows.Close()
		if err != nil {
			log.Error("Message: ", err.Error())
			rows.Close()
		}
		err = rows.Err()
		if err != nil {
			log.Error("Message: ", err.Error())
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
		// log.Debug("Body:", string(body))
	}
	// log.Debug("Products:", products)
	return products
}

// prints out the products slice
func printProducts(products product) (page int, link string) {

	var skulist []sku
	var tempsku sku
	for i := range products.Data {
		tempsku.SKU = products.Data[i].Sku
		tempsku.Qty = products.Data[i].InventoryLevel
		tempsku.ID = products.Data[i].ID
		tempsku.Factory = products.Data[i].Brand_ID
		tempsku.SupplySKU = products.Data[i].MPN
		tempsku.Modified = products.Data[i].Modified
		tempsku.Price = products.Data[i].Price
		if len(products.Data[i].Images) > 0 {
			tempsku.Skuimage = products.Data[i].Images[0]
		}
		// log.Debug("tempsku: ", tempsku)
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
	//log.Debug(mindate())
	mindate := mindate() //"2023-01-01"
	log.Debug(mindate)
	//link = "?include_fields=sku,inventory_level,inventory_warning_level,custom_fields&inventory_level=0&limit="+strconv.Itoa(limit)+"&date_modified:min="+mindate
	link = "?include_fields=sku,inventory_level,inventory_warning_level,mpn,brand_id,date_modified,price&include=images&limit=" + strconv.Itoa(limit) + "&date_modified:min=" + mindate
	url = "https://api.bigcommerce.com/stores/" + storeid + "/v3/catalog/products"

	//Loop through the pages
	totalpages := jsonLoad(urlmake(url, link)).Meta.Pagination.TotalPages
	log.Debug("Total Pages:", totalpages)
	// Initialize progress bar
	log.Info("Updating QTY")
	uiprogress.Start()                                                      // start rendering
	bar := uiprogress.AddBar(totalpages).AppendCompleted().PrependElapsed() // add a new bar
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return "Processing: " // prepend the current processing state
	})
	i := 0
	for i < totalpages {
		bar.Incr() //update progress bar
		log.Info("Processing Page ", i)
		page, newlink := printProducts(jsonLoad(urlmake(url, link)))
		log.Debug("Next Page Query:", page, newlink)
		link = newlink
		i = page
	}
	// log.Debug("Final Data:", skulist)
	// csvmake()
}
