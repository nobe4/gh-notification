package jq

import (
	"fmt"

	"github.com/itchyny/gojq"
	"github.com/nobe4/gh-not/internal/notifications"
)

// TODO: refactor this as a callback to be called on n.Filter(flt) and have
// n.Filter call .Compact
// Filter applies a `.[] | select(filter)` on the notifications.
func Filter(filter string, n notifications.Notifications) (notifications.Notifications, error) {
	if filter == "" {
		return n, nil
	}

	// Extract the IDs from the notifications that match the filter.
	// The actual selection will be done by the parent.
	query, err := gojq.Parse(fmt.Sprintf(".[] | select(%s) | .id", filter))
	if err != nil {
		return nil, err
	}

	// gojq works only on `any` data, so we need to convert Notifications to
	// interface{}. This also gives us back the JSON fields from the API.
	notificationsRaw, err := n.Marshall()
	if err != nil {
		return nil, err
	}

	filteredIDs := []string{}

	iter := query.Run(notificationsRaw)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if err, ok := v.(error); ok {
			if err, ok := err.(*gojq.HaltError); ok && err.Value() == nil {
				break
			}
			return nil, err
		}

		newId, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("invalid filtered id %#v", v)
		}

		filteredIDs = append(filteredIDs, newId)
	}

	return n.FilterFromIds(filteredIDs), nil
}
