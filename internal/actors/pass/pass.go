package pass

import "github.com/nobe4/gh-not/internal/notifications"

type Actor struct{}

func (_ *Actor) Run(n *notifications.Notification) (string, error) {
	return "", nil
}
