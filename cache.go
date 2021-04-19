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

	for _, itemCode := range itemCodes {
		// TODO: parallelize this, it can be optimized to not make the calls to the external service sequentially
		// price, err := c.GetPriceFor(itemCode)
		// if err != nil {
		// 	return []float64{}, err
		// }
		wg.Add(1)

		go func(itemCode string) {
			price, _ := c.GetPriceFor(itemCode)
			prices <- price
			wg.Done()
		}(itemCode)
		// results = append(results, price)
	}

	wg.Wait()
	close(prices)

	for price := range prices {
		results = append(results, price)
	}

	return results, nil
}

func (c *TransparentCache) isCacheAlive() bool {
	elapsed := time.Now().Sub(time.Unix(0, 0)).Milliseconds() - c.startAge.Milliseconds()
	if elapsed > c.maxAge.Milliseconds() {
		return false
	}
	return true
}

func (c *TransparentCache) resetStartAge() {
	c.startAge = time.Duration(time.Now().UnixNano())
}
