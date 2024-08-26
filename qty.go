package main

import (
	// "encoding/csv"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"

	// "log"
	"net/http"
	"os"
	"strconv"
	"time"

	"strings"

	"github.com/gosuri/uiprogress"
	"github.com/sirupsen/logrus"
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

// Customer Structs
// Define the structs for the JSON structure
type Address struct {
	ID              int           `json:"id"`
	Address1        string        `json:"address1"`
	Address2        string        `json:"address2"`
	AddressType     string        `json:"address_type"`
	City            string        `json:"city"`
	Company         string        `json:"company"`
	Country         string        `json:"country"`
	CountryCode     string        `json:"country_code"`
	CustomerID      int           `json:"customer_id"`
	FirstName       string        `json:"first_name"`
	LastName        string        `json:"last_name"`
	Phone           string        `json:"phone"`
	PostalCode      string        `json:"postal_code"`
	StateOrProvince string        `json:"state_or_province"`
	FormFields      []interface{} `json:"form_fields"` // Assuming form fields can be anything; adjust as necessary
}

type Authentication struct {
	ForcePasswordReset bool `json:"force_password_reset"`
}

type StoreCredit struct {
	Amount float64 `json:"amount"`
}

type Customer struct {
	ID                                      int            `json:"id"`
	AddressCount                            int            `json:"address_count"`
	Addresses                               []Address      `json:"addresses"`
	Attributes                              []interface{}  `json:"attributes"` // Assuming attributes can be anything; adjust as necessary
	Authentication                          Authentication `json:"authentication"`
	Company                                 string         `json:"company"`
	CustomerGroupID                         int            `json:"customer_group_id"`
	Email                                   string         `json:"email"`
	FirstName                               string         `json:"first_name"`
	LastName                                string         `json:"last_name"`
	Notes                                   string         `json:"notes"`
	Phone                                   string         `json:"phone"`
	RegistrationIPAddress                   string         `json:"registration_ip_address"`
	TaxExemptCategory                       string         `json:"tax_exempt_category"`
	DateCreated                             string         `json:"date_created"`
	DateModified                            string         `json:"date_modified"`
	AttributeCount                          int            `json:"attribute_count"`
	FormFields                              []interface{}  `json:"form_fields"` // Assuming form fields can be anything; adjust as necessary
	AcceptsProductReviewAbandonedCartEmails bool           `json:"accepts_product_review_abandoned_cart_emails"`
	StoreCreditAmounts                      []StoreCredit  `json:"store_credit_amounts"`
	OriginChannelID                         int            `json:"origin_channel_id"`
	ChannelIDs                              []int          `json:"channel_ids"` // Nullable, so it could be nil
}

type PaginationLinks struct {
	Previous string `json:"previous"`
	Current  string `json:"current"`
	Next     string `json:"next"`
}

type MetaPagination struct {
	Total       int             `json:"total"`
	Count       int             `json:"count"`
	PerPage     int             `json:"per_page"`
	CurrentPage int             `json:"current_page"`
	TotalPages  int             `json:"total_pages"`
	Links       PaginationLinks `json:"links"`
}

type Meta struct {
	Pagination MetaPagination `json:"pagination"`
}

type CustomersResponse struct {
	Data []Customer `json:"data"`
	Meta Meta       `json:"meta"`
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

func customers() {
	// Open connection to database
	log.Info("Opening DB Connection")
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/purchasing"
	db, err := sql.Open("mysql", connectstring)
	if err != nil {
		log.Error("Error opening Database: ", err.Error())
		return
	}
	defer db.Close()

	// Test Connection
	pingErr := db.Ping()
	if pingErr != nil {
		log.Error("Error testing DB Connection: ", pingErr.Error())
		return
	}

	// Define URL strings
	var url string
	storeid := os.Getenv("BIGCOMMERCE_STOREID")
	limit := 250

	// Define the Request URL with the correct query parameters
	url = fmt.Sprintf("https://api.bigcommerce.com/stores/%s/v3/customers?include=addresses,storecredit,attributes,formfields,shopper_profile_id,segment_ids", storeid)

	// Initialize progress bar (will be set after the first API call when we know total pages)
	var bar *uiprogress.Bar

	// Initialize page counter
	page := 1

	for {
		// Construct the URL for the current page
		apiUrl := fmt.Sprintf("%s&limit=%d&page=%d", url, limit, page)
		customersResponse, err := getCustomers(apiUrl)
		if err != nil {
			log.Fatalf("Error fetching data for page %d: %v", page, err)
		}

		// Debug output for pagination metadata
		log.Printf("Page %d Pagination Metadata: TotalPages=%d, CurrentPage=%d, PerPage=%d, Total=%d, Count=%d\n",
			page,
			customersResponse.Meta.Pagination.TotalPages,
			customersResponse.Meta.Pagination.CurrentPage,
			customersResponse.Meta.Pagination.PerPage,
			customersResponse.Meta.Pagination.Total,
			customersResponse.Meta.Pagination.Count)

		// If this is the first page, initialize the progress bar
		if page == 1 {
			totalpages := customersResponse.Meta.Pagination.TotalPages
			log.Println("Total Pages:", totalpages)

			uiprogress.Start()                                                     // start rendering
			bar = uiprogress.AddBar(totalpages).AppendCompleted().PrependElapsed() // add a new bar
			bar.PrependFunc(func(b *uiprogress.Bar) string {
				return "Processing: " // prepend the current processing state
			})
		}

		// Process the customer data
		printCustomers(customersResponse, db)

		// Update the progress bar
		bar.Incr()
		log.Println("Processed Page", page)

		// Check if there are more pages to fetch
		if page >= customersResponse.Meta.Pagination.TotalPages {
			break // No more pages to fetch
		}

		// Increment the page number for the next iteration
		page++
	}

	uiprogress.Stop() // stop the progress bar when done
	log.Println("Customer data processing complete.")
}

// Add Customers to the Database
func printCustomers(customersResponse *CustomersResponse, db *sql.DB) {
	const batchSize = 100

	baseCustomerSQL := `
		REPLACE INTO orders.customers_bc (
			CustomerID, Email, FirstName, LastName, Company, Phone, Notes,
			TaxExemptCategory, CustomerGroupID, RegistrationIPAddress, DateCreated, DateModified,
			AcceptsProductReviewAbandonedCartEmails, OriginChannelID
		) VALUES `

	baseAddressSQL := `
		REPLACE INTO orders.customer_addresses_bc (
			ID, CustomerID, FirstName, LastName, Address1, Address2, City,
			StateOrProvince, PostalCode, CountryCode, Phone, AddressType, Country
		) VALUES `

	customerValueStrings := make([]string, 0, batchSize)
	addressValueStrings := make([]string, 0, batchSize)

	customerValueArgs := make([]interface{}, 0, batchSize*14)
	addressValueArgs := make([]interface{}, 0, batchSize*13)

	for i, customer := range customersResponse.Data {
		// Debug log for customer being processed
		logrus.Debugf("Processing customer: ID=%d, Email=%s", customer.ID, customer.Email)
		// Convert DateCreated and DateModified to MySQL format
		dateCreated, err := formatDateForMySQL(customer.DateCreated)
		if err != nil {
			logrus.Errorf("Error parsing DateCreated for customer ID %d: %v", customer.ID, err)
			continue
		}
		dateModified, err := formatDateForMySQL(customer.DateModified)
		if err != nil {
			logrus.Errorf("Error parsing DateModified for customer ID %d: %v", customer.ID, err)
			continue
		}

		customerValueStrings = append(customerValueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		customerValueArgs = append(customerValueArgs,
			customer.ID, customer.Email, customer.FirstName, customer.LastName, customer.Company,
			customer.Phone, customer.Notes, customer.TaxExemptCategory, customer.CustomerGroupID,
			customer.RegistrationIPAddress, dateCreated, dateModified,
			customer.AcceptsProductReviewAbandonedCartEmails, customer.OriginChannelID,
		)

		// Debug log showing the customer values
		logrus.Debugf("Customer values: %v", customerValueArgs[len(customerValueArgs)-14:])

		for _, address := range customer.Addresses {
			// Debug log for address being processed
			logrus.Debugf("Processing address: ID=%d, CustomerID=%d", address.ID, customer.ID)

			addressValueStrings = append(addressValueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
			addressValueArgs = append(addressValueArgs,
				address.ID, customer.ID, address.FirstName, address.LastName, address.Address1,
				address.Address2, address.City, address.StateOrProvince, address.PostalCode, address.CountryCode,
				address.Phone, address.AddressType, address.Country,
			)

			// Debug log showing the address values
			logrus.Debugf("Address values: %v", addressValueArgs[len(addressValueArgs)-13:])
		}

		if (i+1)%batchSize == 0 || i+1 == len(customersResponse.Data) {
			if len(customerValueStrings) > 0 {
				customerSQL := baseCustomerSQL + strings.Join(customerValueStrings, ",")
				// logrus.Debugf("Executing SQL for customers: %s", customerSQL)
				_, err := db.Exec(customerSQL, customerValueArgs...)
				if err != nil {
					logrus.Errorf("Error inserting batch of customers: %v", err)
				} else {
					logrus.Debugf("Successfully inserted/updated batch of %d customers", len(customerValueStrings))
				}
			}

			if len(addressValueStrings) > 0 {
				addressSQL := baseAddressSQL + strings.Join(addressValueStrings, ",")
				// logrus.Debugf("Executing SQL for addresses: %s", addressSQL)
				_, err := db.Exec(addressSQL, addressValueArgs...)
				if err != nil {
					logrus.Errorf("Error inserting batch of addresses: %v", err)
				} else {
					logrus.Debugf("Successfully inserted/updated batch of %d addresses", len(addressValueStrings))
				}
			}

			customerValueStrings = customerValueStrings[:0]
			customerValueArgs = customerValueArgs[:0]
			addressValueStrings = addressValueStrings[:0]
			addressValueArgs = addressValueArgs[:0]
		}
	}
}

// getCustomers fetches customer data from the API and unmarshals it into the CustomersResponse struct
func getCustomers(apiUrl string) (*CustomersResponse, error) {
	// Define the Request Client
	commerceClient := http.Client{
		Timeout: time.Second * 20, // Timeout after 20 seconds
	}

	// Create the HTTP Request
	req, err := http.NewRequest(http.MethodGet, apiUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Setup Header
	req.Header.Set("User-Agent", "commerce-client")
	req.Header.Add("x-auth-token", os.Getenv("BIGCOMMERCE_TOKEN"))

	// Perform the Request
	res, getErr := commerceClient.Do(req)
	if getErr != nil {
		return nil, fmt.Errorf("failed to perform request: %v", getErr)
	}
	defer res.Body.Close()

	// Read the response body
	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response body: %v", readErr)
	}

	// Unmarshal the JSON data into the CustomersResponse struct
	var customersResponse CustomersResponse
	err = json.Unmarshal(body, &customersResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	return &customersResponse, nil
}

// Insert BigCommerce Orders into the database
func printOrders(ordersResponse []Order, db *sql.DB) {
	const batchSize = 250

	baseOrderSQL := `
		REPLACE INTO orders.orders_bc (
			ID, CustomerID, DateCreated, DateModified, DateShipped, StatusID, Status,
			SubtotalExTax, SubtotalIncTax, SubtotalTax, BaseShippingCost, ShippingCostExTax, ShippingCostIncTax, ShippingCostTax, ShippingCostTaxClassID,
			BaseHandlingCost, HandlingCostExTax, HandlingCostIncTax, HandlingCostTax, HandlingCostTaxClassID,
			BaseWrappingCost, WrappingCostExTax, WrappingCostIncTax, WrappingCostTax, WrappingCostTaxClassID,
			TotalExTax, TotalIncTax, TotalTax, ItemsTotal, ItemsShipped,
			PaymentMethod, PaymentProviderID, PaymentStatus, RefundedAmount, OrderIsDigital, StoreCreditAmount,
			GiftCertificateAmount, IPAddress, IPAddressV6, GeoipCountry, GeoipCountryISO2, CurrencyID,
			CurrencyCode, CurrencyExchangeRate, DefaultCurrencyID, DefaultCurrencyCode, StaffNotes, CustomerMessage,
			DiscountAmount, CouponDiscount
		) VALUES `

	orderValueStrings := make([]string, 0, batchSize)
	orderValueArgs := make([]interface{}, 0, batchSize*52)

	logrus.Debug("Parsing Dates")

	for i, order := range ordersResponse {
		// Convert dates to MySQL format
		dateCreated, err := formatDateForMySQL(order.DateCreated)
		if err != nil {
			logrus.Errorf("Error parsing DateCreated for order ID %d: %v", order.ID, err)
			continue
		}
		dateModified, err := formatDateForMySQL(order.DateModified)
		if err != nil {
			logrus.Errorf("Error parsing DateModified for order ID %d: %v", order.ID, err)
			continue
		}
		dateShipped, err := formatDateForMySQL(order.DateShipped)
		if err != nil {
			logrus.Errorf("Error parsing DateShipped for order ID %d: %v", order.ID, err)
			continue
		}

		//Debug
		orderValueStrings = append(orderValueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		orderValueArgs = append(orderValueArgs,
			order.ID, order.CustomerID, dateCreated, dateModified, dateShipped, order.StatusID, order.Status,
			order.SubtotalExTax, order.SubtotalIncTax, order.SubtotalTax, order.BaseShippingCost, order.ShippingCostExTax, order.ShippingCostIncTax, order.ShippingCostTax, order.ShippingCostTaxClassID,
			order.BaseHandlingCost, order.HandlingCostExTax, order.HandlingCostIncTax, order.HandlingCostTax, order.HandlingCostTaxClassID,
			order.BaseWrappingCost, order.WrappingCostExTax, order.WrappingCostIncTax, order.WrappingCostTax, order.WrappingCostTaxClassID,
			order.TotalExTax, order.TotalIncTax, order.TotalTax, order.ItemsTotal, order.ItemsShipped,
			order.PaymentMethod, order.PaymentProviderID, order.PaymentStatus, order.RefundedAmount, order.OrderIsDigital, order.StoreCreditAmount,
			order.GiftCertificateAmount, order.IPAddress, order.IPAddressV6, order.GeoipCountry, order.GeoipCountryISO2, order.CurrencyID,
			order.CurrencyCode, order.CurrencyExchangeRate, order.DefaultCurrencyID, order.DefaultCurrencyCode, order.StaffNotes, order.CustomerMessage,
			order.DiscountAmount, order.CouponDiscount,
		)

		// Debug log showing the order values
		// logrus.Debugf("Order values: %v", orderValueArgs[len(orderValueArgs)-52:])

		if (i+1)%batchSize == 0 || i+1 == len(ordersResponse) {
			if len(orderValueStrings) > 0 {
				orderSQL := baseOrderSQL + strings.Join(orderValueStrings, ",")
				// logrus.Debugf("Executing SQL for orders: %s", orderSQL)
				logrus.Debugf("Executing SQL for orders:")
				_, err := db.Exec(orderSQL, orderValueArgs...)
				if err != nil {
					logrus.Errorf("Error inserting batch of orders: %v", err)
				} else {
					logrus.Debugf("Successfully inserted/updated batch of %d orders", len(orderValueStrings))
				}
			}

			orderValueStrings = orderValueStrings[:0]
			orderValueArgs = orderValueArgs[:0]
		}
	}
}
