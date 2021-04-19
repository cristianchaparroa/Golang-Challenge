package sample1

import (
	"fmt"
	"sync"
	"time"
)

// PriceService is a service that we can use to get prices for the items
// Calls to this service are expensive (they take time)
type PriceService interface {
	GetPriceFor(itemCode string) (float64, error)
}

// TransparentCache is a cache that wraps the actual service
// The cache will remember prices we ask for, so that we don't have to wait on every call
// Cache should only return a price if it is not older than "maxAge", so that we don't get stale prices
type TransparentCache struct {
	actualPriceService PriceService
	startAge           time.Duration
	maxAge             time.Duration
	prices             map[string]float64
}

func NewTransparentCache(actualPriceService PriceService, maxAge time.Duration) *TransparentCache {
	return &TransparentCache{
		actualPriceService: actualPriceService,
		startAge:           time.Duration(time.Now().UnixNano()),
		maxAge:             maxAge,
		prices:             map[string]float64{},
	}
}

// GetPriceFor gets the price for the item, either from the cache or the actual service if it was not cached or too old
func (c *TransparentCache) GetPriceFor(itemCode string) (float64, error) {
	price, ok := c.prices[itemCode]
	if ok {
		if c.isCacheAlive() {
			return price, nil
		} else {
			c.resetStartAge()
		}
	}
	price, err := c.actualPriceService.GetPriceFor(itemCode)
	if err != nil {
		return 0, fmt.Errorf("getting price from service : %v", err.Error())
	}
	c.prices[itemCode] = price
	return price, nil
}

// GetPricesFor gets the prices for several items at once, some might be found in the cache, others might not
// If any of the operations returns an error, it should return an error as well
func (c *TransparentCache) GetPricesFor(itemCodes ...string) ([]float64, error) {
	var results []float64

	wg := &sync.WaitGroup{}
	prices := make(chan float64, len(itemCodes))
	errors := make(chan error, len(itemCodes))

	for _, itemCode := range itemCodes {
		wg.Add(1)
		go c.GetParallelPriceFor(itemCode, prices, errors, wg)
	}

	wg.Wait()
	close(prices)
	close(errors)

	for err := range errors {
		return results, err
	}

	for price := range prices {
		results = append(results, price)
	}

	return results, nil
}
// isCacheAlive verifies if the elapsed time is before to the maxAge to query the
// prices from services or cache.
func (c *TransparentCache) isCacheAlive() bool {
	elapsed := time.Now().Sub(time.Unix(0, 0)).Milliseconds() - c.startAge.Milliseconds()
	return elapsed < c.maxAge.Milliseconds()
}

// resetStartAge assigns the current time to the startAge.  
func (c *TransparentCache) resetStartAge() {
	c.startAge = time.Duration(time.Now().UnixNano())
}
// GetParallelPriceFor retrieves the prices using channels.
func (c *TransparentCache) GetParallelPriceFor(itemCode string, prices chan float64, errors chan error, wg *sync.WaitGroup) {
	price, err := c.GetPriceFor(itemCode)
	if err != nil {
		errors <- err
		wg.Done()
	}
	prices <- price
	wg.Done()
}
