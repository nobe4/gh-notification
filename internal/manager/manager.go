package manager

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/nobe4/gh-not/internal/actors"
	"github.com/nobe4/gh-not/internal/api"
	"github.com/nobe4/gh-not/internal/cache"
	"github.com/nobe4/gh-not/internal/config"
	"github.com/nobe4/gh-not/internal/gh"
	"github.com/nobe4/gh-not/internal/notifications"
)

type RefreshStrategy int

const (
	DefaultRefresh RefreshStrategy = iota
	ForceRefresh
	ForceNoRefresh
)

type Manager struct {
	Notifications notifications.Notifications
	cache         cache.ExpiringReadWriter
	config        *config.Data
	client        *gh.Client
	Actors        actors.ActorsMap
	refresh       RefreshStrategy
}

func New(config *config.Data, caller api.Caller, refresh, noRefresh bool) *Manager {
	m := &Manager{}

	m.config = config
	m.cache = cache.NewFileCache(m.config.Cache.TTLInHours, m.config.Cache.Path)
	m.client = gh.NewClient(caller, m.cache, m.config.Endpoint)
	m.Actors = actors.Map(m.client)

	m.setRefresh(refresh, noRefresh)

	return m
}

func (m *Manager) setRefresh(refresh, noRefresh bool) {
	m.refresh = DefaultRefresh
	if refresh {
		m.refresh = ForceRefresh
	} else if noRefresh {
		m.refresh = ForceNoRefresh
	}
}

func (m *Manager) Load() error {
	allNotifications := notifications.Notifications{}

	cachedNotifications, expired, err := m.loadCache()
	if err != nil {
		slog.Warn("cannot read the cache: %#v\n", err)
	} else if cachedNotifications != nil {
		allNotifications = cachedNotifications
	}

	if m.shouldRefresh(expired) {
		fmt.Printf("Refreshing the cache...\n")

		remoteNotifications, err := m.client.Notifications()
		if err != nil {
			return err
		}

		allNotifications = notifications.Sync(allNotifications, remoteNotifications)

		if err := m.cache.Write(allNotifications); err != nil {
			slog.Error("Error while writing the cache: %#v", err)
		}
	}

	m.Notifications = allNotifications.Uniq()

	return nil
}

func (m *Manager) shouldRefresh(expired bool) bool {
	if !expired && m.refresh == ForceRefresh {
		slog.Info("forcing a refresh")
		return true
	}

	if expired && m.refresh == ForceNoRefresh {
		slog.Info("preventing a refresh")
		return false
	}

	slog.Debug("refresh", "refresh", expired)
	return expired
}

func (m *Manager) Save() error {
	return m.cache.Write(m.Notifications.Compact())
}

func (m *Manager) loadCache() (notifications.Notifications, bool, error) {
	expired, err := m.cache.Expired()
	if err != nil {
		return nil, false, err
	}

	n := notifications.Notifications{}
	if err := m.cache.Read(&n); err != nil {
		return nil, expired, err
	}

	return n, expired, nil
}

func (m *Manager) Apply(noop bool) error {
	for _, rule := range m.config.Rules {
		actor, ok := m.Actors[rule.Action]
		if !ok {
			slog.Error("unknown action", "action", rule.Action)
			continue
		}

		selectedIds, err := rule.FilterIds(m.Notifications)
		if err != nil {
			return err
		}

		slog.Debug("apply rule", "name", rule.Name, "count", len(selectedIds))

		for _, notification := range m.Notifications.FilterFromIds(selectedIds) {
			if notification.Meta.Done {
				continue
			}

			if noop {
				fmt.Printf("NOOP'ing action %s on notification %s\n", rule.Action, notification.String())
				continue
			}

			if err := actor.Run(notification, os.Stdout); err != nil {
				slog.Error("action failed", "action", rule.Action, "err", err)
			}
			fmt.Fprintln(os.Stdout, "")
		}
	}

	m.Notifications = m.Notifications.Compact()

	return nil
}
