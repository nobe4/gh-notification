// Interact with GitHub's api, wrapper around cli/go-gh client object.
package gh

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"

	ghapi "github.com/cli/go-gh/v2/pkg/api"
	"github.com/nobe4/gh-not/internal/api"
	"github.com/nobe4/gh-not/internal/cache"
	"github.com/nobe4/gh-not/internal/notifications"
)

const (
	DefaultEndpoint = "https://api.github.com/notifications"
	retryCount      = 5
)

type Client struct {
	API   api.Caller
	cache cache.ExpiringReadWriter
}

func NewClient(api api.Caller, cache cache.ExpiringReadWriter) *Client {
	return &Client{
		API:   api,
		cache: cache,
	}
}

func (c *Client) loadCache() (notifications.Notifications, bool, error) {
	expired, err := c.cache.Expired()
	if err != nil {
		return nil, false, err
	}

	n := notifications.Notifications{}
	if err := c.cache.Read(&n); err != nil {
		return nil, expired, err
	}

	return n, expired, nil
}

func isRetryable(err error) bool {
	var httpError *ghapi.HTTPError

	if errors.As(err, &httpError) {
		switch httpError.StatusCode {
		case 502, 504:
			return true
		}
	}

	return false
}

func (c *Client) retryRequest(verb, endpoint string, body io.Reader) (*http.Response, error) {
	for i := retryCount; i > 0; i-- {
		response, err := c.API.Request(verb, endpoint, body)
		if err != nil {
			if isRetryable(err) {
				slog.Warn("endpoint failed with retryable status", "endpoint", endpoint, "retry left", i)
				continue
			}

			return nil, err
		}

		return response, nil
	}

	return nil, fmt.Errorf("retry exceeded for %s", endpoint)
}

// inspired by https://github.com/cli/go-gh/blob/25db6b99518c88e03f71dbe9e58397c4cfb62caf/example_gh_test.go#L96-L134
func (c *Client) paginateNotifications() (notifications.Notifications, error) {
	var list notifications.Notifications

	var linkRE = regexp.MustCompile(`<([^>]+)>;\s*rel="([^"]+)"`)
	findNextPage := func(response *http.Response) string {
		for _, m := range linkRE.FindAllStringSubmatch(response.Header.Get("Link"), -1) {
			if len(m) > 2 && m[2] == "next" {
				return m[1]
			}
		}
		return ""
	}

	endpoint := DefaultEndpoint
	for endpoint != "" {
		slog.Info("API REST request", "endpoint", endpoint)

		response, err := c.retryRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}

		pageList := notifications.Notifications{}
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(&pageList)
		if err != nil {
			return nil, err
		}

		list = append(list, pageList...)

		if err := response.Body.Close(); err != nil {
			return nil, err
		}

		endpoint = findNextPage(response)
	}

	return list, nil
}

func (c *Client) pullNotificationFromApi() (notifications.Notifications, error) {
	list, err := c.paginateNotifications()
	if err != nil {
		return nil, err
	}

	for i, n := range list {
		if err := c.enrichNotification(n); err != nil {
			return nil, err
		}

		list[i] = n
	}

	return list, nil
}

func (c *Client) Notifications() (notifications.Notifications, error) {
	allNotifications := notifications.Notifications{}

	pulledNotifications, err := c.pullNotificationFromApi()
	if err != nil {
		return nil, err
	}

	allNotifications = append(allNotifications, pulledNotifications...)

	if err := c.cache.Write(allNotifications); err != nil {
		slog.Error("Error while writing the cache: %#v", err)
	}

	return allNotifications.Uniq(), nil
}
