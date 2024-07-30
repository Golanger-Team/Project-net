package main

/*
This file will contain methods needed to fetch ads from panel,
together with its metadata from PostgreSQL database.

The result will be written to a `map` structure that maps a pair of (ad-id, publisher-id)
to a pair of (impression count, CTR).
*/

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
	CTR         int
}

// An interval in which we believe that the estimated CTR probably lies.
// This probability is a hyper-parameter that should be tuned manually.
// (A conventional value is 95%)
type ConfidenceInterval struct {
	lowerBound float32
	upperBound float32
}

// Maps collaborations to their empirical success statistics.
var evaluation map[AdPublisherCollaboration]Statistics

// Indicates a range containing the emprical CTR in which the real CTR will most likely lie.
var toleranceRange map[AdPublisherCollaboration]ConfidenceInterval

// Maps id of each advertiser to mean ctr of its ads.
var meanCtr map[AdvertiserPublisherCollaboration]int

// What revenue is expected to gain from showing ad x to publisher y?
var expectedRevenue map[AdPublisherCollaboration]float32

// The per-publisher distribution of ads that affects the selection process.
var weight map[AdPublisherCollaboration]float32

/* Functions Used for Updating Ad Statistics */

/* Queries the metadata database and retrieves each advertiser's mean CTR per publisher. */
func fetchMeanCTRs() {
	// TODO: Update meanCtr
}

/* Queries the metadata database and computes the success statistics of each ad-publisher pair. */
func fetchAdStatistics() {
	// TODO: Update success statistics
}

/* For each publisher, sets CTR of its new ads to the mean CTR of its advertiser. */
func usePriorsForNewAds() {
	// TODO
}

/*
Updates the expected gain revenue for each ad-publisher pair by
taking the product of the ad's bid value and the ad-publisher pair's
estimated CTR.
*/
func updateExpectedRevenues() {
	// TODO
}

/*
Calculates a confidence interval for each ad-publisher estimated CTR.
The method relies in part on the Hoeffding’s inequality.
*/
func calculateToleranceRanges() {
	// TODO
}

/*
Re-calculates per-publisher distributions on ads based on
the computed confidence intervals for ad-publisher pairs.
*/
func updatePerPublisherDistributions() {
	// TODO
}
