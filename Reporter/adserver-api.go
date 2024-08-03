package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

const REPORTER_PORT = 9999
const MEAN_CTR_API = "/mean_ctr"
const AD_PUBLISHER_API = "/ad_publisher"

type AdvertiserPublisherEventCount struct {
	AdvertiserId	string
	PublisherId		string
	EventType		string
	Total			int
}

type AdPublisherEventCount struct {
	AdId			string
	PublisherId		string
	EventType		string
	Total			int
}

// A struct reflecting the collaboration of an ad with a publisher.
type AdPublisherCollaboration struct {
	AdID        int
	PublisherID int
}

type AdvertiserPublisherCollaboration struct {
	AdvertiserID int
	PublisherID  int
}

type CTRArrayEntry struct {
	Collaboration	AdvertiserPublisherCollaboration
	Stat			Statistics
}

type AdPublisherEntry struct {
	Collaboration	AdPublisherCollaboration
	Stat			Statistics
}

// Stores the statistics of a collaboration. Namely, impression count, click count and ctr.
type Statistics struct {
	Impressions int
	Clicks      int
	CTR         float64
}

// Maps advertiser-publisher collaborations to their emprical success statistics.
var advertiserEvaluation  = make(map[AdvertiserPublisherCollaboration]Statistics)

// Maps ad-publisher collaborations to ther emprical success statistics.
var adEvaluation = make(map[AdPublisherCollaboration]Statistics)
var eventCounts []AdvertiserPublisherEventCount

/* Sends the mean ctr of each advertiser's ads, per publisher. */
func sendAdvertisersMeanCTR(c *gin.Context) {
	var timeCondition = "time > now() - INTERVAL '1 hour'"
	db.Table("events").Select("advertiser_id, publisher_id, event_type, count(1) AS total").Where(timeCondition).Group("advertiser_id, publisher_id, event_type").Scan(&eventCounts)
	fmt.Printf("eventCounts: %v\n", eventCounts)
	var collaboration AdvertiserPublisherCollaboration
	for _, eventCount := range eventCounts {
		var statistics Statistics
		collaboration.AdvertiserID, _ = strconv.Atoi(eventCount.AdvertiserId)
		collaboration.PublisherID,  _ = strconv.Atoi(eventCount.PublisherId)
		
		statistics = advertiserEvaluation[collaboration]
		switch eventCount.EventType {
		case "impression":
			statistics.Impressions = eventCount.Total
		case "click":
			statistics.Clicks = eventCount.Total
		default:
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		advertiserEvaluation[collaboration] = statistics
	}

	/* Compute CTR, together with fixing possible inconsistencies
	 in data. These inconsistencies can happen, for example by
	 latency in arrival of click and impression events. */
	var entries []CTRArrayEntry
	for apc := range advertiserEvaluation {
		var statistics = advertiserEvaluation[apc]
		if statistics.Impressions < statistics.Clicks {
			statistics.Impressions = statistics.Clicks
		}
		if statistics.Impressions > 0 {
			statistics.CTR = float64(statistics.Clicks) / float64(statistics.Impressions)
		}
		advertiserEvaluation[apc] = statistics
		var newEntry CTRArrayEntry
		newEntry.Collaboration = apc
		newEntry.Stat = statistics
		entries = append(entries, newEntry)
	}
	
	/* Our statistics map is now ready to be sent. */
	fmt.Printf("Array is ready to be sent: %v\n", entries)
	c.JSON(http.StatusOK, entries)
}


/* Sends the per-publisher success statistics of each Ad. */
func sendAdStatistics(c *gin.Context) {
	var timeCondition = "time > now() - INTERVAL '1 hour'"
	
	var eventCounts []AdPublisherEventCount
	db.Table("events").Select("ad_id, publisher_id, event_type, count(1) AS total").Where(timeCondition).Group("ad_id, publisher_id, event_type").Scan(&eventCounts)

	var collaboration AdPublisherCollaboration
	for _, eventCount := range eventCounts {
		collaboration.AdID, _ = strconv.Atoi(eventCount.AdId)
		collaboration.PublisherID, _ = strconv.Atoi(eventCount.PublisherId)

		var statistics = adEvaluation[collaboration]
		switch eventCount.EventType {
		case "impression":
			statistics.Impressions = eventCount.Total
		case "click":
			statistics.Clicks = eventCount.Total
		default:
			c.AbortWithStatus(http.StatusInternalServerError)
		}
		adEvaluation[collaboration] = statistics
	}
	/* Compute CTR, together with fixing possible inconsistencies
	 in data. These inconsistencies can happen, for example by
	 latency in arrival of click and impression events. */
	var entries []AdPublisherEntry
	for apc := range adEvaluation {
		var statistics = adEvaluation[apc]
		if statistics.Impressions < statistics.Clicks {
			statistics.Impressions = statistics.Clicks
		}
		if statistics.Impressions > 0 {
			statistics.CTR = float64(statistics.Clicks) / float64(statistics.Impressions)
		}
		adEvaluation[apc] = statistics
		var newEntry AdPublisherEntry
		newEntry.Collaboration = apc
		newEntry.Stat = statistics
		entries = append(entries, newEntry)
	}

	/* Our statistics map is now ready to be sent. */
	c.JSON(http.StatusOK, entries)
}

/* Runs the router that will route api calls from ad server to
 handlers. Note that this function block the calling goroutine
 indefinitely. */
func setupAndRunAPIRouter() {
	router := gin.Default()
	router.GET(MEAN_CTR_API, sendAdvertisersMeanCTR)
	router.GET(AD_PUBLISHER_API, sendAdStatistics)

	router.Run(":" + strconv.Itoa(REPORTER_PORT))
}