package main

import (
	"fmt"
	"net/http"
	"io/ioutil"
	"os"
	"encoding/json"
	"strconv"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	// "time"
	"github.com/golang-module/carbon/v2"
)

type order []struct {
	// Data []struct {
		ID                    int    `json:"id"`
		Status_ID             int    `json:"status_id"`
		Date_created          string `json:"date_created"`
		Items_total           int `json:"items_total"`
		Order_total						string `json:"total_ex_tax"`
}

type orderdetail struct {
	ID int
	Status_ID int
	Date_created string
	Items_total int
	Order_total string
}

var orderlist []orderdetail

func dateconvert(newdate string) (f string){
		return carbon.ParseByFormat(newdate).ToDateTimeString()
}

func minorder() (val int){
	//open connection to database
		connectstring := os.Getenv("USER")+":"+os.Getenv("PASS")+"@tcp("+os.Getenv("SERVER")+":"+os.Getenv("PORT")+")/orders"
		db, err := sql.Open("mysql",
		connectstring)
		if err != nil {
			fmt.Println("Message: ",err.Error())
		}

		//Test Connection
		pingErr := db.Ping()
		if pingErr != nil {
			fmt.Println("Message: ",err.Error())
		}

		var testquery string = "SELECT MAX(id) from orders"
    rows2, err := db.Query(testquery)
    if err != nil {
      fmt.Println(err.Error())
    }
    // var val uint
    if rows2.Next() {
      rows2.Scan(&val)
    }
		fmt.Println(val)
		return val
}

func orderinsert(orders order) {

//open connection to database
	connectstring := os.Getenv("USER")+":"+os.Getenv("PASS")+"@tcp("+os.Getenv("SERVER")+":"+os.Getenv("PORT")+")/orders"
	db, err := sql.Open("mysql",
	connectstring)
	if err != nil {
		fmt.Println("Message: ",err.Error())
	}

	//Test Connection
	pingErr := db.Ping()
	if pingErr != nil {
		fmt.Println("Message: ",err.Error())
	}

	for i := range orders {
		var newquery string = "INSERT INTO `orders`(`id`,`statusid`,`date_created`,`items_total`,`order_total`) VALUES (?,?,?,?,?)"
		rows, err := db.Query(newquery,orders[i].ID,orders[i].Status_ID,orders[i].Date_created,orders[i].Items_total,orders[i].Order_total)
		if err != nil {
			fmt.Println("Message: ",err.Error())
		}
		err = rows.Err()
		if err != nil {
			fmt.Println("Message: ",err.Error())
		}
	}
}

func main() {
	fmt.Println("Setting URL...")
	limit := "100"

	fmt.Println("Finding starting order...")
	minid := minorder() + 1
	// minid = 66717
	url := "https://api.bigcommerce.com/stores/"+os.Getenv("BIGCOMMERCE_STOREID")+"/v2/orders?min_id="+strconv.Itoa(minid)+"&sort=id:asc&limit="+limit

	fmt.Println("Creating Request...")
	req, _ := http.NewRequest("GET", url, nil)

	fmt.Println("Setting Headers...")
	// req.Header.Add("Content-Type", "application/json")
	req.Header.Set("User-Agent", "commerce-client")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-Auth-Token", os.Getenv("BIGCOMMERCE_TOKEN"))

	fmt.Println("Executing Request...")
	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	fmt.Println("Creating Orders Structure...")
	orders:=order{}

	fmt.Println("Loading JSON...")
	jsonErr := json.Unmarshal(body, &orders)
	if jsonErr != nil {
		fmt.Println(jsonErr.Error())
		fmt.Println("Body:",string(body))
	}

	fmt.Println("Inserting Orders...")
	// orderinsert(orders)

	var temporder orderdetail
	for i := range orders {
			temporder.ID = orders[i].ID
			temporder.Items_total=orders[i].Items_total
			temporder.Status_ID=orders[i].Status_ID
			temporder.Date_created=orders[i].Date_created
			temporder.Order_total=orders[i].Order_total
			orderlist = append(orderlist,temporder)
			fmt.Println("ID:"+strconv.Itoa(temporder.ID)+" Total: "+strconv.Itoa(temporder.Items_total)+" Status_ID: "+strconv.Itoa(temporder.Status_ID)+" Date Created: "+dateconvert(temporder.Date_created))
	}

	fmt.Println("Final Data: ",orderlist)

	// fmt.Println(res)
	// fmt.Println(string(body))

}
