package service

import (
	"errors"
	"github.com/sony/gobreaker"
	"net/http"
	"time"
)

type ExternalAPIClient struct {
	cb *gobreaker.CircuitBreaker
}

func NewExternalAPIClient() *ExternalAPIClient {
	settings := gobreaker.Settings{
		Name:        "ExternalAPI",
		MaxRequests: 5,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
	}

	return &ExternalAPIClient{cb: gobreaker.NewCircuitBreaker(settings)}
}

func (c *ExternalAPIClient) CallAPI(url string) (*http.Response, error) {
	result, err := c.cb.Execute(func() (interface{}, error) {
		resp, err := http.Get(url)
		if err != nil || resp.StatusCode >= 500 {
			return nil, errors.New("API call failed")
		}
		return resp, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*http.Response), nil
}
