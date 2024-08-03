package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
)

/*
This file will contain methods needed to fetch ads from panel,
together with its metadata from PostgreSQL database.

The result will be written to a `map` structure that maps a pair of (ad-id, publisher-id)
to a pair of (impression count, CTR).


IN THE FOLLOWING, A DEMO OF THE FUNCTIONALITIES
IMPLEMENTED IN THIS FILE IS DESCRIBED


* When fetching ads from panel:
	* fetch ad success statistics from metadata database
	* compute the CTR per publisher of every ad
		A SELECT statement --> map[ad-id][publisher-id] -> (impression, ctr)
		A SELECT statement --> map[advertiser-id] -> mean CTR
	* if an ad is not displayed on this publisher yet, set CTR to the average CTR of ads of its advertiser
		for on fetched-ad, publisher: if map[ad][publisher] = null: map[ad][publisher] = (0, mean CTR of advertiser)
	* compute the revenue expected value by taking the product of CTR and bid, for each ad
		expected[ad-id][publisher-id] = bid[ad-id]  * prob[ad-id][publisher-id]
	* compute the tolerance range for each ad
		algorithm described below
	* select the ad with maximum expected value, together with ads that can 'beat' that ad as a result of tolerance.
		---
	* compute the relative weights of those selected ads
		weight[ad-id][publisher-id] = expected[ad-id][publisher-id] / NORMALIZATION_TERM
* When a publisher requests for a new ad:
	* draw a random weighted ad, encrypt and send.
		Maybe using the uniform distribution?


* Confidence interval (95%):

** METHOD ONE: ADDITION-BASED
P (d <= e) > 1 - 2exp(-2e^2N) >= 0.95
-2e^2N <= ln(0.025)
N >=  ln(40)/(2e^2) ~= 1.8444 / e^2
e^2 >= ln(40)/(2N)
e >= sqrt(ln(40) / 2)  /  sqrt(N) ~= 1.36 / sqrt(N)
upper bound: CTR + 1.36 / sqrt(N)
lower bound: CTR - 1.36 / sqrt(N)

** METHOD TWO: MULTIPLICATION-BASED
upper bound: CTR * a
lower bound: CTR / a
*/

/* Constants Configuring Functionality of Ad-Fetching */
const MEAN_CTR_API = "/mean_ctr"
const AD_PUBLISHER_API = "/ad_publisher"

/* Structs and Variables Relating to Ad-fetching. */

// A struct reflecting the collaboration of an ad with a publisher.
type AdPublisherCollaboration struct {
	AdID        int
	PublisherID int
}

type AdvertiserPublisherCollaboration struct {
	AdvertiserID int
	PublisherID  int
}

// Stores the statistics of a collaboration. Namely, impression count, click count and ctr.
type Statistics struct {
	Impressions int
	Clicks      int
	CTR         float64
}

type CTRArrayEntry struct {
	Collaboration	AdvertiserPublisherCollaboration
	Stat			Statistics
}

type AdPublisherEntry struct {
	Collaboration	AdPublisherCollaboration
	Stat			Statistics
}


// An interval in which we believe that the estimated CTR probably lies.
// This probability is a hyper-parameter that should be tuned manually.
// (A conventional value is 95%)
type ConfidenceInterval struct {
	lowerBound float64
	upperBound float64
}

// Maps collaborations to their emprical success statistics.
var adEvaluation = make(map[AdPublisherCollaboration]Statistics)

// Maps id of each advertiser to mean ctr of its ads.
var advertiserEvaluation = make(map[AdvertiserPublisherCollaboration]Statistics)

// Indicates a range containing the emprical CTR in which the real CTR will most likely lay.
var toleranceRange = make(map[AdPublisherCollaboration]ConfidenceInterval)

// What revenue is expected to gain from showing ad x to publisher y?
var expectedRevenue = make(map[AdPublisherCollaboration]float64)

// The per-publisher distribution of ads that affects the selection process.
var weight = make(map[AdPublisherCollaboration]float64)

// To some extent can the actual CTR be different, relative to the estimated CTR.
// In other words, it is assumed that actual ctr most likely lays in the interval
// [estimated_ctr / RELATIVE_TOLERACE, estimated_ctr * RELATIVE_TOLERANCE]
const RELATIVE_TOLERANCE = 2

var meanCtrEntries []CTRArrayEntry
var adPubEntries []AdPublisherEntry

/* Functions Used for Updating Ad Statistics */

/* Issues a GET request to the specified url. Returnes the 
 response a slice of bytes, together with errors encountered
 in the process, if any. */
func getRequest(getUrl string) ([]byte, error) {
	client := http.DefaultClient
	req, err := http.NewRequest("GET", getUrl, nil)
	if err != nil {
		log.Println("error in making request")
		return nil, err
	}
	
	resp, err := client.Do(req)
	if err != nil {
		log.Println("error in doing request")
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Println("error in Reporter")
		return nil, errors.New("reporter sent " + resp.Status)
	}
	responseByte, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("error in reading response body")
		return nil, err
	}
	return responseByte, nil
}

/* Makes a request to Reporter and retrieves each advertiser's mean CTR per publisher. */
func fetchMeanCTRs() error {
	responseByte, err := getRequest(MEAN_CTR_API)
	if err != nil {
		return err
	}
	err = json.Unmarshal(responseByte, &meanCtrEntries)
	if err != nil {
		log.Println("error in parsing response")
		return err
	}
	for apc := range advertiserEvaluation {
		delete(advertiserEvaluation, apc)
	}
	/* Update the map according to recieved entries. */
	var collaboration AdvertiserPublisherCollaboration
	var statistics Statistics
	for _,  meanCtrEntry := range meanCtrEntries {
		collaboration = meanCtrEntry.Collaboration
		statistics = meanCtrEntry.Stat
		advertiserEvaluation[collaboration] = statistics
	}
	return nil
}

/* Queries the metadata database and computes the success statistics of each ad-publisher pair. */
func fetchAdStatistics() error {
	responseByte, err := getRequest(MEAN_CTR_API)
	if err != nil {
		return err
	}
	err = json.Unmarshal(responseByte, &adPubEntries)
	if err != nil {
		log.Println("error in parsing response")
		return err
	}
	for apc := range adEvaluation {
		delete(adEvaluation, apc)
	}
	/* Update the map according to recieved entries. */
	var collaboration AdPublisherCollaboration
	var statistics Statistics
	for _, adPubEntry := range adPubEntries {
		collaboration = adPubEntry.Collaboration
		statistics = adPubEntry.Stat
		adEvaluation[collaboration] = statistics
	}
	return nil
}

/* For each publisher, sets CTR of its new ads to the mean CTR of its advertiser. */
func usePriorsForNewAds() {
	var adPubCollab AdPublisherCollaboration
	var statistics Statistics
	var exists bool
	var advertiserPubCollab AdvertiserPublisherCollaboration

	for _, publisherID := range allPublisherIDs {
		adPubCollab.PublisherID = publisherID
		for _, ad := range allFetchedAds {
			adPubCollab.AdID = ad.Id
			statistics, exists = adEvaluation[adPubCollab]
			if !exists || statistics.Impressions == 0 {
				statistics.Impressions = 0
				statistics.Clicks = 0
				advertiserPubCollab.AdvertiserID = ad.AdvertiserID
				advertiserPubCollab.PublisherID = publisherID
				statistics.CTR = advertiserEvaluation[advertiserPubCollab].CTR
				adEvaluation[adPubCollab] = statistics
			}
		}
	}
}

/* Updates the expected gain revenue for each ad-publisher pair by
 taking the product of the ad's bid value and the ad-publisher pair's
 estimated CTR. */
func updateExpectedRevenues() {
	var adPubCollab AdPublisherCollaboration

	for _, publisherID := range allPublisherIDs {
		adPubCollab.PublisherID = publisherID
		for _, ad := range allFetchedAds {
			adPubCollab.AdID = ad.Id
			expectedRevenue[adPubCollab] = float64(ad.Bid) * adEvaluation[adPubCollab].CTR
		}
	}
}

/* Calculates a confidence interval for each ad-publisher estimated CTR.
 The method relies in part on the Hoeffding’s inequality. */
func calculateToleranceRanges() {
	var adPubCollab AdPublisherCollaboration
	var absoluteConfInterval ConfidenceInterval
	var relativeConfInterval ConfidenceInterval
	var finalConfInterval ConfidenceInterval
	var ctr float64
	var N int

	for _, publisherID := range allPublisherIDs {
		adPubCollab.PublisherID = publisherID
		for _, ad := range allFetchedAds {
			adPubCollab.AdID = ad.Id
			ctr = adEvaluation[adPubCollab].CTR
			N = adEvaluation[adPubCollab].Impressions
			if N > 0 {
				absoluteConfInterval.upperBound = ctr + 1.36 / math.Sqrt(float64(N))
				absoluteConfInterval.lowerBound = ctr - 1.36 / math.Sqrt(float64(N))
			} else {
				absoluteConfInterval.upperBound = 1	
				absoluteConfInterval.lowerBound = 0
			}

			relativeConfInterval.upperBound = ctr * RELATIVE_TOLERANCE
			relativeConfInterval.lowerBound = ctr / RELATIVE_TOLERANCE
			finalConfInterval.upperBound = min(absoluteConfInterval.upperBound, relativeConfInterval.upperBound)
			finalConfInterval.lowerBound = max(absoluteConfInterval.lowerBound, relativeConfInterval.lowerBound)
			toleranceRange[adPubCollab] = finalConfInterval
		}
	}
}


/* Re-calculates per-publisher distributions on ads based on
the computed confidence intervals for ad-publisher pairs. */
func updatePerPublisherDistributions() {
	var adPubCollab AdPublisherCollaboration
	var maxLowerBound float64
	var winnerAdsRevenueSum float64

	for _, publisherID := range allPublisherIDs {
		adPubCollab.PublisherID = publisherID
		maxLowerBound = 0
		for _, ad := range allFetchedAds {
			adPubCollab.AdID = ad.Id
			if toleranceRange[adPubCollab].lowerBound > maxLowerBound {
				maxLowerBound =  toleranceRange[adPubCollab].lowerBound
			}
		}
		winnerAdsRevenueSum = 0
		for _, ad := range allFetchedAds {
			adPubCollab.AdID = ad.Id
			if toleranceRange[adPubCollab].upperBound >= maxLowerBound {
				winnerAdsRevenueSum += expectedRevenue[adPubCollab]
			}
		}
		for _, ad := range allFetchedAds {
			adPubCollab.AdID = ad.Id
			if toleranceRange[adPubCollab].upperBound >= maxLowerBound {
				weight[adPubCollab] = expectedRevenue[adPubCollab] / winnerAdsRevenueSum
			}
		}
	}
}