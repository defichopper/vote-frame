package notifications

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vocdoni/vote-frame/farcasterapi"
	"github.com/vocdoni/vote-frame/mongo"
	"go.vocdoni.io/dvote/log"
)

const (
	DefaultListenCoolDown = 30 * time.Second
	DefaultSendCoolDown   = 500 * time.Millisecond
	NotificationMessage   = `👋 Hey @%s!

The user %s created a new poll!

🗳 And you're eligible to vote!

Cast your vote to make a difference 👇`
)

type NotificationManager struct {
	ctx            context.Context
	cancel         context.CancelFunc
	db             *mongo.MongoStorage
	api            farcasterapi.API
	listenCoolDown time.Duration
}

func New(ctx context.Context, db *mongo.MongoStorage, api farcasterapi.API, listenCoolDown time.Duration) *NotificationManager {
	ctx, cancel := context.WithCancel(ctx)
	return &NotificationManager{
		ctx:            ctx,
		cancel:         cancel,
		db:             db,
		api:            api,
		listenCoolDown: listenCoolDown,
	}
}

func (nm *NotificationManager) Start() {
	go func() {
		for {
			select {
			case <-nm.ctx.Done():
				return
			case <-time.After(nm.listenCoolDown):
				notifications, err := nm.db.LastNotifications(100)
				if err != nil {
					log.Errorf("error getting notifications: %s", err)
					continue
				}
				log.Infow("notifications found", "count", len(notifications))
				if err := nm.sendNotifications(notifications); err != nil {
					log.Errorf("error sending notifications: %s", err)
				}
			}
		}
	}()
}

func (nm *NotificationManager) Stop() {
	nm.cancel()
}

func (nm *NotificationManager) sendNotifications(notifications []mongo.Notification) error {
	// create channels and waitgroup, the semaphore is used to limit the number
	// of concurrent goroutines and the error channel is used to return any
	// error found
	sem := make(chan struct{}, 10)
	errCh := make(chan error, 1)
	wg := sync.WaitGroup{}
	// iterate over notifications and send them
	for _, n := range notifications {
		// add goroutine to waitgroup and semaphore
		wg.Add(1)
		sem <- struct{}{}
		go func(n mongo.Notification) {
			defer wg.Done()
			defer func() { <-sem }()
			// send notification and remove it from the database
			msg := fmt.Sprintf(NotificationMessage, n.Username, n.AuthorUsername)
			if err := nm.api.Publish(nm.ctx, msg, []uint64{n.UserID}, n.FrameUrl); err != nil {
				errCh <- fmt.Errorf("error sending notification: %s", err)
				return
			}
			if err := nm.db.RemoveNotification(n.ID); err != nil {
				errCh <- fmt.Errorf("error deleting notification: %s", err)
				return
			}
		}(n)

		time.Sleep(DefaultSendCoolDown)
	}
	// wait for all goroutines to finish and close channels
	go func() {
		wg.Wait()
		close(errCh)
		close(sem)
	}()
	// listen error channel and return any err error found
	for err := range errCh {
		return err
	}
	// return nil if no error is found
	return nil
}
