package pgx

/*
remove if not used
import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Listen listens (`LISTEN channel_name;`) on a given channel for notifications (triggered by `NOTIFY channel_name, 'payload_string';`)
// and returns channels of notifications and errors. It runs in a goroutine and listens until the context is canceled.
func Listen(ctx context.Context, conn Conn, channelName string) (<-chan *pgconn.Notification, <-chan error) {
	notifications := make(chan *pgconn.Notification)
	errors := make(chan error)

	var waitForNotification func(context.Context) (*pgconn.Notification, error)

	if poolConn, ok := conn.(*pgxpool.Conn); ok {
		waitForNotification = poolConn.Conn().WaitForNotification
	} else if pgxConn, ok := conn.(*pgx.Conn); ok {
		waitForNotification = pgxConn.WaitForNotification
	} else {
		errors <- fmt.Errorf("connection must be *pgxpool.Conn or *pgx.Conn")
		close(notifications)
		close(errors)
		return notifications, errors
	}

	if _, err := conn.Exec(ctx, "LISTEN "+channelName); err != nil {
		errors <- fmt.Errorf("error listening to channel: %w", err)
		close(notifications)
		close(errors)
		return notifications, errors
	}

	go func() {
		defer close(notifications)
		defer close(errors)

		for {
			select {
			case <-ctx.Done():
				errors <- ctx.Err()
				return
			default:
				notification, err := waitForNotification(ctx)
				if err != nil {
					errors <- err
					continue
				}
				if notification != nil {
					notifications <- notification
				}
			}
		}
	}()

	return notifications, errors
}
*/
