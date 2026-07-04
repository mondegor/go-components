package session

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Границы клампа отклонений soft/hard дублируются здесь (minSessionThreshold/maxSessionThreshold,
// тип int) и в composition-root (wire/mrauth/config, тип int8). Оба набора неэкспортируемы,
// поэтому сравнить напрямую нельзя; вместо этого обе стороны пришпилены к одному
// документированному эталону -4/16. Парный тест на стороне config -
// TestSessionThresholdBounds_MirrorDomain (wire/mrauth/config). Любая односторонняя правка
// ломает соответствующий тест и заставляет синхронизировать вторую сторону.
const (
	expectedMinSessionThreshold = -4
	expectedMaxSessionThreshold = 16
)

func TestSessionThresholdBounds_MirrorConfig(t *testing.T) {
	t.Parallel()

	require.Equal(t, expectedMinSessionThreshold, minSessionThreshold)
	require.Equal(t, expectedMaxSessionThreshold, maxSessionThreshold)
}
