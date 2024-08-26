package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gosuri/uiprogress"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

const (
	maxRetries    = 3                // Maximum number of retries
	retryInterval = 10 * time.Second // Time to wait between retries
)

type Order struct {
	ID                     int    `json:"id"`
	CustomerID             int    `json:"customer_id"`
	DateCreated            string `json:"date_created"`
	DateModified           string `json:"date_modified"`
	DateShipped            string `json:"date_shipped"`
	StatusID               int    `json:"status_id"`
	Status                 string `json:"status"`
	SubtotalExTax          string `json:"subtotal_ex_tax"`
	SubtotalIncTax         string `json:"subtotal_inc_tax"`
	SubtotalTax            string `json:"subtotal_tax"`
	BaseShippingCost       string `json:"base_shipping_cost"`
	ShippingCostExTax      string `json:"shipping_cost_ex_tax"`
	ShippingCostIncTax     string `json:"shipping_cost_inc_tax"`
	ShippingCostTax        string `json:"shipping_cost_tax"`
	ShippingCostTaxClassID int    `json:"shipping_cost_tax_class_id"`
	BaseHandlingCost       string `json:"base_handling_cost"`
	HandlingCostExTax      string `json:"handling_cost_ex_tax"`
	HandlingCostIncTax     string `json:"handling_cost_inc_tax"`
	HandlingCostTax        string `json:"handling_cost_tax"`
	HandlingCostTaxClassID int    `json:"handling_cost_tax_class_id"`
	BaseWrappingCost       string `json:"base_wrapping_cost"`
	WrappingCostExTax      string `json:"wrapping_cost_ex_tax"`
	WrappingCostIncTax     string `json:"wrapping_cost_inc_tax"`
	WrappingCostTax        string `json:"wrapping_cost_tax"`
	WrappingCostTaxClassID int    `json:"wrapping_cost_tax_class_id"`
	TotalExTax             string `json:"total_ex_tax"`
	TotalIncTax            string `json:"total_inc_tax"`
	TotalTax               string `json:"total_tax"`
	ItemsTotal             int    `json:"items_total"`
	ItemsShipped           int    `json:"items_shipped"`
	PaymentMethod          string `json:"payment_method"`
	PaymentProviderID      string `json:"payment_provider_id"`
	PaymentStatus          string `json:"payment_status"`
	RefundedAmount         string `json:"refunded_amount"`
	OrderIsDigital         bool   `json:"order_is_digital"`
	StoreCreditAmount      string `json:"store_credit_amount"`
	GiftCertificateAmount  string `json:"gift_certificate_amount"`
	IPAddress              string `json:"ip_address"`
	IPAddressV6            string `json:"ip_address_v6"`
	GeoipCountry           string `json:"geoip_country"`
	GeoipCountryISO2       string `json:"geoip_country_iso2"`
	CurrencyID             int    `json:"currency_id"`
	CurrencyCode           string `json:"currency_code"`
	CurrencyExchangeRate   string `json:"currency_exchange_rate"`
	DefaultCurrencyID      int    `json:"default_currency_id"`
	DefaultCurrencyCode    string `json:"default_currency_code"`
	StaffNotes             string `json:"staff_notes"`
	CustomerMessage        string `json:"customer_message"`
	DiscountAmount         string `json:"discount_amount"`
	CouponDiscount         string `json:"coupon_discount"`
}

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

	var testquery string = "SELECT MAX(id) from orders_bc"
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

func fetchOrders(baseURL string) error {
	log.Info("Fetching Orders from BigCommerce")

	// Open connection to database
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/orders"
	db, err := sql.Open("mysql", connectstring)
	if err != nil {
		log.Error("Error opening database: ", err.Error())
		return err
	}
	defer db.Close()

	// Test Connection
	if pingErr := db.Ping(); pingErr != nil {
		log.Error("Error pinging database: ", pingErr.Error())
		return err
	}

	page := 1
	limit := 250 // Adjust as needed, this is the number of orders per page

	for {
		// Construct the URL with pagination
		// baseURL := "https://api.bigcommerce.com/stores/" + os.Getenv("BIGCOMMERCE_STOREID") + "/v2/orders"
		url := fmt.Sprintf("%s&page=%d&limit=%d&sort=id:asc", baseURL, page, limit)
		var ordersResponse []Order

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Error("Error creating new request for page ", page, ": ", err)
			return err
		}

		// Set Headers
		req.Header.Set("User-Agent", "commerce-client")
		req.Header.Add("Accept", "application/json")
		req.Header.Add("X-Auth-Token", os.Getenv("BIGCOMMERCE_TOKEN"))

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Error("Error executing request for page ", page, ": ", err)
			return err
		}
		defer res.Body.Close()

		// Check for 404 Not Found or other errors indicating no more pages
		if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusNoContent {
			log.Info("No more pages to fetch. Stopping.")
			break
		} else if res.StatusCode != http.StatusOK {
			log.Error("Error response from server for page ", page, ": Status ", res.Status)
			return fmt.Errorf("server returned non-OK status: %s", res.Status)
		}

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Error("Error reading response body for page ", page, ": ", err)
			return err
		}

		// Unmarshal JSON into ordersResponse
		if jsonErr := json.Unmarshal(body, &ordersResponse); jsonErr != nil {
			log.Error("Error unmarshalling JSON for page ", page, ": ", jsonErr)
			return jsonErr
		}

		// If no orders are returned, assume we have fetched all pages
		if len(ordersResponse) == 0 {
			log.Info("No more orders to fetch. Stopping.")
			break
		}

		// Process and insert the orders into the database
		printOrders(ordersResponse, db)

		// Increment the page number
		page++
	}

	log.Info("Successfully fetched all orders.")
	return nil
}

func formatDateForMySQL(dateStr string) (interface{}, error) {
	if dateStr == "" {
		logrus.Debug("Date string is empty, returning NULL")
		return nil, nil // Return nil to represent NULL in MySQL
	}

	// Try parsing the date in RFC1123Z format first
	parsedTime, err := time.Parse(time.RFC1123Z, dateStr)
	if err == nil {
		// logrus.Debugf("Successfully parsed date '%s' using RFC1123Z", dateStr)
		return parsedTime.Format("2006-01-02 15:04:05"), nil
	} else {
		// logrus.Debugf("Failed to parse date '%s' using RFC1123Z: %v", dateStr, err)
	}

	// If the first format fails, try RFC3339 format
	parsedTime, err = time.Parse(time.RFC3339, dateStr)
	if err == nil {
		// logrus.Debugf("Successfully parsed date '%s' using RFC3339", dateStr)
		return parsedTime.Format("2006-01-02 15:04:05"), nil
	} else {
		// logrus.Debugf("Failed to parse date '%s' using RFC3339: %v", dateStr, err)
	}

	// If neither format worked, return an error
	// logrus.Debugf("Unsupported date format for date string: '%s'", dateStr)
	return nil, errors.New("unsupported date format: " + dateStr)
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
	minid := minorder() + 1
	// minid := 0
	log.Debug("Minimum order ID: ", minid)

	url := "https://api.bigcommerce.com/stores/" + os.Getenv("BIGCOMMERCE_STOREID") + "/v2/orders?min_id=" + strconv.Itoa(minid)
	log.Debug("URL: ", url)

	// orders, err := fetchOrders(url)
	// if err != nil {
	// 	log.Error("Failed to fetch orders: ", err)
	// } else {

	// 	log.Info("Inserting Orders...")
	// 	orderinsert(orders)
	// }

	if err := fetchOrders(url); err != nil {
		log.Fatal("Failed to fetch and process orders: ", err)
	}

	SSLoad()
	qty()
	customers()
	log.Info("Completed update")
}
