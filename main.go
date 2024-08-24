package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gosuri/uiprogress"
	log "github.com/sirupsen/logrus"
)

const (
	maxRetries    = 3                // Maximum number of retries
	retryInterval = 10 * time.Second // Time to wait between retries
)

type order []struct {
	ID             int    `json:"id"`
	Status_ID      int    `json:"status_id"`
	Date_created   string `json:"date_created"`
	Items_total    int    `json:"items_total"`
	Order_total    string `json:"total_ex_tax"`
	BillingAddress struct {
		Email string `json:"email"`
	} `json:"billing_address"`
	CustomerID int `json:"customer_id"`
}

type orderdetail struct {
	ID           int
	Status_ID    int
	Date_created time.Time
	Items_total  int
	Order_total  string
	Email        string
	SKUS         []sku
	CustomerID   int
}

var orderlist []orderdetail

// minorder retrieves the maximum order ID from the database.
func minorder() (val int) {
	// Open connection to the database
	log.Info("Connecting to DB")
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/orders"
	db, err := sql.Open("mysql", connectstring)
	if err != nil {
		log.Error("Error opening database: ", err.Error())
		return
	}
	defer db.Close()

	// Test Connection
	if pingErr := db.Ping(); pingErr != nil {
		log.Error("Error pinging database: ", pingErr.Error())
		return
	}

	var testquery string = "SELECT MAX(id) from orders"
	rows2, err := db.Query(testquery)
	if err != nil {
		log.Error("Error querying database: ", err.Error())
		return
	}
	defer rows2.Close()

	if rows2.Next() {
		if err := rows2.Scan(&val); err != nil {
			log.Error("Error scanning row: ", err.Error())
		}
	}
	log.Debug("Max order ID: ", val)
	return val
}

// orderinsert inserts the fetched orders into the database.
func orderinsert(orders order) {

	if len(orders) == 0 {
		log.Info("No Orders in Slice")
		return
	}
	// Open connection to database
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/orders"
	db, err := sql.Open("mysql", connectstring)
	if err != nil {
		log.Error("Error opening database: ", err.Error())
		return
	}
	defer db.Close()

	// Test Connection
	if pingErr := db.Ping(); pingErr != nil {
		log.Error("Error pinging database: ", pingErr.Error())
		return
	}
	// Initialize progress bar
	log.Info("Inserting BigCommerce Orders into Database")
	uiprogress.Start()                                                       // start rendering
	bar := uiprogress.AddBar(len(orders)).AppendCompleted().PrependElapsed() // add a new bar
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return "Processing: " // prepend the current processing state
	})
	for _, o := range orders {
		bar.Incr() //update progress bar
		if o.ID == 0 {
			continue
		}
		var newquery string = "REPLACE INTO `orders`(`id`,`statusid`,`date_created`,`items_total`,`order_total`, `email`, `customer_id`) VALUES (?,?,?,?,?,?,?)"
		ordertime, err := time.Parse("Mon, 02 Jan 2006 15:04:05 +0000", o.Date_created)
		if err != nil {
			log.Error("Error parsing date: ", err.Error())
			log.Error("Error on Order: ", o.ID, ",", o.Status_ID, ",", ordertime, ",", o.Items_total, ",", o.Order_total, ",", o.BillingAddress.Email, ",", o.CustomerID)
			continue
		}
		_, err = db.Exec(newquery, o.ID, o.Status_ID, ordertime, o.Items_total, o.Order_total, o.BillingAddress.Email, o.CustomerID)
		if err != nil {
			log.Error("Error executing query: ", err.Error())
			continue
		}
	}
}

// fetchOrders fetches orders from BigCommerce API and applies retry logic.
func fetchOrders(url string) (order, error) {
	log.Info("Fetching Orders from BigCommerce")
	var orders order
	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Error("Error creating new request, attempt ", i+1, " of ", maxRetries, ": ", err)
			time.Sleep(retryInterval)
			continue
		}

		// Set Headers
		req.Header.Set("User-Agent", "commerce-client")
		req.Header.Add("Accept", "application/json")
		req.Header.Add("X-Auth-Token", os.Getenv("BIGCOMMERCE_TOKEN"))

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Error("Error executing request, attempt ", i+1, " of ", maxRetries, ": ", err)
			time.Sleep(retryInterval)
			continue
		}
		defer res.Body.Close()

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Error("Error reading response body, attempt ", i+1, " of ", maxRetries, ": ", err)
			time.Sleep(retryInterval)
			continue
		}

		// Unmarshal JSON
		if jsonErr := json.Unmarshal(body, &orders); jsonErr != nil {
			log.Error("Error unmarshalling JSON, attempt ", i+1, " of ", maxRetries, ": ", jsonErr)
			// log.Debug("Body: ", string(body))
			time.Sleep(retryInterval)
			continue
		}

		return orders, nil
	}
	return orders, fmt.Errorf("failed to fetch orders from BigCommerce after %d attempts", maxRetries)
}

func formatDateForMySQL(dateStr string) (string, error) {
	parsedTime, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return "", err
	}
	return parsedTime.Format("2006-01-02 15:04:05"), nil
}

func main() {
	// Logging configuration
	if os.Getenv("LOGLEVEL") == "DEBUG" {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.Info("Starting BigCommerce Update")

	// limit := "250"
	// minid := minorder() + 1
	// log.Debug("Minimum order ID: ", minid)

	// url := "https://api.bigcommerce.com/stores/" + os.Getenv("BIGCOMMERCE_STOREID") + "/v2/orders?min_id=" + strconv.Itoa(minid) + "&sort=id:asc&limit=" + limit
	// log.Debug("URL: ", url)

	// orders, err := fetchOrders(url)
	// if err != nil {
	// 	log.Error("Failed to fetch orders: ", err)
	// } else {

	// 	log.Info("Inserting Orders...")
	// 	orderinsert(orders)
	// }

	// SSLoad()
	// qty()
	customers()
	log.Info("Completed update")
}
