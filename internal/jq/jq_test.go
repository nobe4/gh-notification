package jq

import (
	"errors"
	"testing"

	"github.com/itchyny/gojq"
	"github.com/nobe4/gh-not/internal/notifications"
)

func notificationsEqual(a notifications.Notifications, ids []string) bool {
	if len(a) != len(ids) {
		return false
	}

	for i := range a {
		if a[i].Id != ids[i] {
			return false
		}
	}

	return true
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		n      notifications.Notifications
		want   []string
		err    error
	}{
		{
			name:   "empty filter",
			filter: "",
			n: notifications.Notifications{
				&notifications.Notification{Id: "0"},
				&notifications.Notification{Id: "1"},
				&notifications.Notification{Id: "2"},
			},
			want: []string{"0", "1", "2"},
		},
		{
			name:   "invalid filter",
			filter: "!!!",
			err:    &gojq.ParseError{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Filter(test.filter, test.n)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected error %#v but got %#v", test.err, err)
			}

			if !notificationsEqual(got, test.want) {
				t.Fatalf("expected %#v but got %#v", test.want, got)
			}
		})
	}
}
