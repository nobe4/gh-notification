package config

import (
	"slices"
	"testing"

	"github.com/nobe4/gh-not/internal/notifications"
)

func TestFilterIds(t *testing.T) {
	tests := []struct {
		r    Rule
		n    notifications.Notifications
		want []string
	}{
		{
			r:    Rule{Name: "no filter"},
			n:    notifications.Notifications{{Id: "0"}, {Id: "1"}, {Id: "2"}},
			want: []string{"0", "1", "2"},
		},

		{
			r: Rule{
				Name:    "filter for a specific id",
				Filters: []string{`.id == "1"`},
			},
			n:    notifications.Notifications{{Id: "0"}, {Id: "1"}, {Id: "2"}},
			want: []string{"1"},
		},

		// Or
		{
			r: Rule{
				Name: "filter for a specific set of ids",
				Filters: []string{
					// All three filters are equivalent
					`.id == "1" or .id == "2"`,
					`(.id == "1" or .id == "2")`,
					`(.id == "1") or (.id == "2")`,
				},
			},
			n:    notifications.Notifications{{Id: "0"}, {Id: "1"}, {Id: "2"}},
			want: []string{"1", "2"},
		},

		// And
		{
			r: Rule{
				Name: "filters are joined by and",
				Filters: []string{
					`.reason == "test"`,
					`.unread == true`,
				},
			},
			n: notifications.Notifications{
				{Id: "0", Reason: "test", Unread: true},
				{Id: "1", Reason: "test", Unread: false},
				{Id: "2", Reason: "ci_activity", Unread: true},
			},
			want: []string{"0"},
		},
		// Can also be written as
		{
			r: Rule{
				Name: "use and in filter",
				Filters: []string{
					`.reason == "test" and .unread == true`,
				},
			},
			n: notifications.Notifications{
				{Id: "0", Reason: "test", Unread: true},
				{Id: "1", Reason: "test", Unread: false},
				{Id: "2", Reason: "ci_activity", Unread: true},
			},
			want: []string{"0"},
		},
	}

	for _, test := range tests {
		t.Run(test.r.Name, func(t *testing.T) {
			got, err := test.r.FilterIds(test.n)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if len(got) != len(test.want) {
				t.Fatalf("want %#v, but got %#v", test.want, got)
			}

			if !slices.Equal(got, test.want) {
				t.Fatalf("want %#v, but got %#v", test.want, got)
			}
		})
	}
}
