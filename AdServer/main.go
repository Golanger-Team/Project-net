package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

/* Constants Configuring the Functionality of Server. */

const FETCH_PERIOD = 5 // How many minutes to wait between fetching
// Ads from Panel.
const FETCH_URL = "http://localhost:8080/api/v1/ads/active"

const PRINT_RESPONSE = true // Wheter to print allAds after it is fetched.

/* User-defined Types and Structs*/

type Ad struct {
	id           string `json:"Id"`
	title        string `json:"Title"`
	redirectLink string `json:"RedirectLink"`
	imageSource  string `json:"ImagePath"`
	bid          int    `json:"BidValue"`
}

/* Global Objects. */

var allAds []Ad // A slice containing all ads.

/* Functions of the Server. */

/* In an infinite loop, waits for `FETCH_PERIOD` minutes
   and then fetches ads from Panel. */
func fetchAds() error {
	for {

		client := http.DefaultClient
		req, err := http.NewRequest("GET", FETCH_URL, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)

		if err != nil {
			return err
		}

		responseByte, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		json.Unmarshal(responseByte, &allAds)

		if PRINT_RESPONSE {
			fmt.Printf("allAds: %v\n", allAds)
		}

		/* Sleep for FETCH_PERIOD minutes. */
		time.Sleep(1 * time.Second) // For demonstration purposes, we just wait
		// a single second instead of FETCH_PERIOD minutes.
	}
}

func selectAd() Ad {
	var bestAd Ad
	var maxBid int = 0

	for _, ad := range allAds {
		if ad.bid > maxBid {
			maxBid = ad.bid
			bestAd = ad
		}
	}
	return bestAd
}

func getNewAd(c *gin.Context) {
	selectedAd := selectAd()
	c.IndentedJSON(http.StatusOK, selectedAd)
}

func main() {
	/* Configure Go's predefined logger. */
	log.SetPrefix("AdServer:")
	log.SetFlags(log.Ltime | log.Ldate)

	/* Run the two main workers: ad-fetcher
	   and query-responser. */
	go fetchAds()
	router := gin.Default()
	router.GET("/new-ad", getNewAd)

	router.Run("localhost:9090")
}
