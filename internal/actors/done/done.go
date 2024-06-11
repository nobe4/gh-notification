package done

import (
	"log/slog"
	"net/http"

	"github.com/nobe4/gh-not/internal/colors"
	"github.com/nobe4/gh-not/internal/gh"
	"github.com/nobe4/gh-not/internal/notifications"
)

// Actor that marks a notification as done.
// Ref: https://docs.github.com/en/rest/activity/notifications?apiVersion=2022-11-28#mark-a-thread-as-done
type Actor struct {
	Client *gh.Client
}

func (a *Actor) Run(n *notifications.Notification) (string, error) {
	slog.Debug("marking notification as done", "notification", n.ToString())

	n.Meta.ToDelete = true

	err := a.Client.API.Do(http.MethodDelete, n.URL, nil, nil)
	if err != nil {
		return "", err
	}

	out := colors.Red("DONE ") + n.ToString()

	return out, nil
}
