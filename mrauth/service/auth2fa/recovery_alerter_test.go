package auth2fa_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mondegor/go-components/mrauth/service/auth2fa"
)

// fakeNotifier - фиксирует факт и параметры отправки уведомления.
type fakeNotifier struct {
	sent  bool
	key   string
	props map[string]any
}

func (n *fakeNotifier) Send(_ context.Context, key string, props map[string]any) error {
	n.sent = true
	n.key = key
	n.props = props

	return nil
}

func TestRecoveryAlerter_AtThresholdSends(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	notifier := &fakeNotifier{}
	alerter := auth2fa.NewRecoveryAlerter(notifier, 2)

	require.NoError(t, alerter.SendAlert(context.Background(), userID, 2)) // остаток == порога
	require.True(t, notifier.sent)
	require.Equal(t, userID, notifier.props["to"]) // получатель резолвится хостом по userID
	require.Equal(t, 2, notifier.props["remaining"])
}

func TestRecoveryAlerter_AboveThresholdSkips(t *testing.T) {
	t.Parallel()

	notifier := &fakeNotifier{}
	alerter := auth2fa.NewRecoveryAlerter(notifier, 2)

	require.NoError(t, alerter.SendAlert(context.Background(), uuid.New(), 3)) // остаток > порога
	require.False(t, notifier.sent)
}
