package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	_ "github.com/go-sql-driver/mysql"

	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"time"
)

const (
	maxRetries    = 3                // Maximum number of retries
	retryInterval = 10 * time.Second // Time to wait between retries
)

type SSOrder struct {
	Data []struct {
		OrderID   string   `json:"orderNumber"`
		Orderskus []SSItem `json:"items"`
	} `json:"orders"`
	Pages int `json:"pages"`
	Page  int `json:"page"`
}

type SSItem struct {
	SKU string `json:"sku"`
	QTY int    `json:"quantity"`
}

// SSLoad loads orders from ShipStation and inserts them into a database.
func SSLoad() {
	limit := 250
	page := 1
	url := "https://ssapi.shipstation.com/orders"
	link := "?orderDateStart=" + mindate() + "&pagesize=" + strconv.Itoa(limit) + "&page=" + strconv.Itoa(page)

	temporder, err := ssjsonload(urlmake(url, link))
	if err != nil {
		log.Error("Error loading JSON from ShipStation: ", err)
		return
	}

	err = ssorderinsert(processorder(temporder))
	if err != nil {
		log.Error("Error inserting orders into the database: ", err)
		return
	}

	for temporder.Page <= temporder.Pages {
		log.Debug("Processing Page: ", page)
		page = temporder.Page + 1
		link = "?orderDateStart=" + mindate() + "&pagesize=" + strconv.Itoa(limit) + "&page=" + strconv.Itoa(page)

		for i := 0; i < maxRetries; i++ {
			temporder, err = ssjsonload(urlmake(url, link))
			if err == nil {
				break
			}
			log.Error("Error loading JSON from ShipStation, attempt ", i+1, " of ", maxRetries, ": ", err)
			time.Sleep(retryInterval)
		}

		if err != nil {
			log.Error("Failed to load JSON from ShipStation after ", maxRetries, " attempts: ", err)
			break
		}
	}
}

// ssorderinsert inserts the processed orders into the database.
func ssorderinsert(orders []orderdetail) error {
	// Open connection to the database
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/orders"
	db, err := sql.Open("mysql", connectstring)
	if err != nil {
		return fmt.Errorf("error opening database: %v", err)
	}
	defer db.Close()

	// Test the database connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("error pinging database: %v", err)
	}

	// Update each order in the database
	for _, order := range orders {
		newquery := "UPDATE `orders` SET ss_qty = ? WHERE id = ?"
		_, err := db.Exec(newquery, order.Items_total, order.ID)
		if err != nil {
			log.Error("Error executing query: ", err)
		}
	}

	return nil
}

// processorder processes the orders from ShipStation.
func processorder(ssorder SSOrder) (orders []orderdetail) {
	for _, order := range ssorder.Data {
		log.Debug("Processing Order: ", order.OrderID)
		var temporder orderdetail
		temporder.ID, _ = strconv.Atoi(order.OrderID)
		temporder.Items_total = 0
		for _, item := range order.Orderskus {
			temporder.Items_total += item.QTY
			log.Debug(temporder.ID, "/", item.SKU, "/", item.QTY, "/", temporder.Items_total)
		}
		orders = append(orders, temporder)
	}
	return orders
}

// ssjsonload makes a GET request to the ShipStation API and returns the unmarshalled JSON.
func ssjsonload(url string) (SSOrder, error) {
	var Orders SSOrder

	for i := 0; i < maxRetries; i++ {
		// Define the Request Client
		method := "GET"
		client := &http.Client{}
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			log.Error("Error creating new request, attempt ", i+1, " of ", maxRetries, ": ", err)
			time.Sleep(retryInterval)
			continue
		}

		// Authorization
		data := []byte(os.Getenv("SSKEY") + ":" + os.Getenv("SSSECRET"))
		dst := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
		base64.StdEncoding.Encode(dst, data)
		log.Debug("Auth: ", string(dst))

		req.Header.Add("Host", "ssapi.shipstation.com")
		req.Header.Add("Authorization", "Basic "+string(dst))

		// Perform the request
		res, err := client.Do(req)
		if err != nil {
			log.Error("Error executing request, attempt ", i+1, " of ", maxRetries, ": ", err)
			time.Sleep(retryInterval)
			continue
		}

		// Read and close the response body
		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close() // Close the body when reading is done
		if err != nil {
			log.Error("Error reading response body, attempt ", i+1, " of ", maxRetries, ": ", err)
			time.Sleep(retryInterval)
			continue
		}

		// Unmarshal JSON
		if jsonErr := json.Unmarshal(body, &Orders); jsonErr != nil {
			log.Error("Error unmarshalling JSON, attempt ", i+1, " of ", maxRetries, ": ", jsonErr)
			time.Sleep(retryInterval)
			continue
		}

		return Orders, nil // Success, return the orders
	}

	// All retries have been exhausted, return the error
	return Orders, fmt.Errorf("failed to load data from ShipStation after %d attempts", maxRetries)
}
