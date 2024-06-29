package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gosuri/uiprogress"

	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	// "github.com/golang-module/carbon/v2"
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

func SSLoad() {

	log.Info("Connecting to Shipstation API")
	limit := 250
	page := 1
	url := "https://ssapi.shipstation.com/orders"
	link := "?orderDateStart=" + mindate() + "&pagesize=" + strconv.Itoa(limit) + "&page=" + strconv.Itoa(page)

	temporder := ssjsonload(urlmake(url, link))

	ssorderinsert(processorder(temporder))

	for temporder.Page <= temporder.Pages {
		log.Debug("Processing Page: ", page)
		page = temporder.Page + 1
		link = "?orderDateStart=" + mindate() + "&pagesize=" + strconv.Itoa(limit) + "&page=" + strconv.Itoa(page)
		temporder = ssjsonload(urlmake(url, link))
		ssorderinsert(processorder(temporder))
	}
}

func ssorderinsert(orders []orderdetail) {

	if len(orders) == 0 {
		log.Info("No Orders in Slice")
		return
	}

	//open connection to database
	log.Info("Opening Connection to the database")
	connectstring := os.Getenv("USER") + ":" + os.Getenv("PASS") + "@tcp(" + os.Getenv("SERVER") + ":" + os.Getenv("PORT") + ")/orders"
	db, err := sql.Open("mysql",
		connectstring)
	if err != nil {
		log.Error("Message: ", err.Error())
	}

	//Test Connection
	pingErr := db.Ping()
	if pingErr != nil {
		log.Error("Message: ", err.Error())
	}

	// Initialize progress bar
	log.Info("Inserting Shipstation Orders into Database")
	uiprogress.Start()                                                       // start rendering
	bar := uiprogress.AddBar(len(orders)).AppendCompleted().PrependElapsed() // add a new bar
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		return "Processing: " // prepend the current processing state
	})

	for i := range orders {
		bar.Incr() //update progress bar
		var newquery string = "UPDATE `orders` SET ss_qty = ? WHERE id = ?"
		rows, err := db.Query(newquery, orders[i].Items_total, orders[i].ID)
		rows.Close()

		if err != nil {
			log.Error("Message: ", err.Error())
		}
		err = rows.Err()
		if err != nil {
			log.Error("Message: ", err.Error())
		}
	}
}

func processorder(ssorder SSOrder) (orders []orderdetail) {
	var temporder orderdetail
	for i := range ssorder.Data {
		// log.Debug("Processing Order: ", ssorder.Data[i].OrderID)
		temporder.ID, _ = strconv.Atoi(ssorder.Data[i].OrderID)
		temporder.Items_total = 0
		for x := range ssorder.Data[i].Orderskus {
			temporder.Items_total += ssorder.Data[i].Orderskus[x].QTY
			// log.Debug(temporder.ID, "/", ssorder.Data[i].Orderskus[x].SKU, "/", ssorder.Data[i].Orderskus[x].QTY, "/", temporder.Items_total)
		}
		orders = append(orders, temporder)
	}
	return orders
}

func ssjsonload(url string) (Orders SSOrder) {
	//Define the Request Client
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		log.Error(err)
	}

	//Authorization
	data := []byte(os.Getenv("SSKEY") + ":" + os.Getenv("SSSECRET"))
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(dst, data)
	log.Debug("Auth: ", string(dst))

	req.Header.Add("Host", "ssapi.shipstation.com")
	req.Header.Add("Authorization", "Basic "+string(dst))

	res, err := client.Do(req)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	// log.Debug("Shipstation JSON: ", string(body))

	//unmarshall JSON
	Orders = SSOrder{}
	jsonErr := json.Unmarshal(body, &Orders)
	if jsonErr != nil {
		log.Fatal(jsonErr)
		// log.Debug("Body:", string(body))
	}
	// log.Debug("Orders:", Orders)
	return Orders
}
