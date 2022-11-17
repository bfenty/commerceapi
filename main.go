package main

import (
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	// "github.com/golang-module/carbon/v2"
)

type order []struct {
	// Data []struct {
	ID           int    `json:"id"`
	Status_ID    int    `json:"status_id"`
	Date_created string `json:"date_created"`
	Items_total  int    `json:"items_total"`
	Order_total  string `json:"total_ex_tax"`
}

type orderdetail struct {
	ID           int
	Status_ID    int
	Date_created time.Time
	Items_total  int
	Order_total  string
}

var orderlist []orderdetail

func minorder() (val int) {
	//open connection to database
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/orders"
	db, err := sql.Open("mysql",
		connectstring)
	if err != nil {
		log.Debug("Message: ", err.Error())
	}

	//Test Connection
	pingErr := db.Ping()
	if pingErr != nil {
		log.Debug("Message: ", err.Error())
	}

	var testquery string = "SELECT MAX(id) from orders"
	rows2, err := db.Query(testquery)
	if err != nil {
		log.Debug(err.Error())
	}
	// var val uint
	if rows2.Next() {
		rows2.Scan(&val)
	}
	log.Debug(val)
	return val
}

func orderinsert(orders order) {

	//open connection to database
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/orders"
	db, err := sql.Open("mysql",
		connectstring)
	if err != nil {
		log.Debug("Message: ", err.Error())
	}

	//Test Connection
	pingErr := db.Ping()
	if pingErr != nil {
		log.Debug("Message: ", err.Error())
	}

	for i := range orders {
		var newquery string = "REPLACE INTO `orders`(`id`,`statusid`,`date_created`,`items_total`,`order_total`) VALUES (?,?,?,?,?)"
		ordertime, _ := time.Parse("Mon, 02 Jan 2006 15:04:05 +0000", orders[i].Date_created)
		rows, err := db.Query(newquery, orders[i].ID, orders[i].Status_ID, ordertime, orders[i].Items_total, orders[i].Order_total)
		if err != nil {
			log.Debug("Message: ", err.Error())
		}
		err = rows.Err()
		if err != nil {
			log.Debug("Message: ", err.Error())
		}
	}
}

func main() {
	if os.Getenv("LOGLEVEL") == "DEBUG" {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.Info("Starting BigCommerce Update")
	log.Debug("Setting URL...")
	limit := "100"
	var err error

	log.Debug("Finding starting order...")
	minid := minorder() + 1
	// minid = 66717
	url := "https://api.bigcommerce.com/stores/" + os.Getenv("BIGCOMMERCE_STOREID") + "/v2/orders?min_id=" + strconv.Itoa(minid) + "&sort=id:asc&limit=" + limit
	log.Debug("URL: ", url)

	log.Debug("Creating Request...")
	req, _ := http.NewRequest("GET", url, nil)

	log.Debug("Setting Headers...")
	req.Header.Set("User-Agent", "commerce-client")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-Auth-Token", os.Getenv("BIGCOMMERCE_TOKEN"))

	log.Debug("Executing Request...")
	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	log.Debug("Creating Orders Structure...")
	orders := order{}

	log.Debug("Loading JSON...")
	jsonErr := json.Unmarshal(body, &orders)
	if jsonErr != nil {
		log.Debug(jsonErr.Error())
		log.Debug("Body:", string(body))
	}

	log.Debug("Inserting Orders...")
	orderinsert(orders)

	var temporder orderdetail
	for i := range orders {
		temporder.ID = orders[i].ID
		temporder.Items_total = orders[i].Items_total
		temporder.Status_ID = orders[i].Status_ID
		temporder.Date_created, err = time.Parse("Mon, 02 Jan 2006 15:04:05 +0000", orders[i].Date_created)
		temporder.Order_total = orders[i].Order_total
		orderlist = append(orderlist, temporder)
		if err != nil {
			log.Debug(err.Error())
		}
		log.Debug("ID:"+strconv.Itoa(temporder.ID)+" Total: "+strconv.Itoa(temporder.Items_total)+" Status_ID: "+strconv.Itoa(temporder.Status_ID)+" Date Created: ", temporder.Date_created)
	}

	//log.Debug("Final Data: ", orderlist)

	SSLoad()
	// qty()
	log.Info("Completed update")
}
