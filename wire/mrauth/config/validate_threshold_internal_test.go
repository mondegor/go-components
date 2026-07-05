package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Границы клампа отклонений soft/hard дублируются в domain-слое
// (mrauth/usecase/session: minSessionThreshold/maxSessionThreshold, тип int) и здесь
// (тип int8). Оба набора неэкспортируемы, поэтому сравнить напрямую нельзя; вместо этого
// обе стороны пришпилены к одному документированному эталону -4/16. Парный тест на стороне
// domain - TestSessionThresholdBounds_MirrorConfig (mrauth/usecase/session). Любая
// односторонняя правка ломает соответствующий тест и заставляет синхронизировать вторую сторону.
const (
	expectedMinSessionThreshold int8 = -4
	expectedMaxSessionThreshold int8 = 16
)

func TestSessionThresholdBounds_MirrorDomain(t *testing.T) {
	t.Parallel()

	require.Equal(t, expectedMinSessionThreshold, minSessionThreshold)
	require.Equal(t, expectedMaxSessionThreshold, maxSessionThreshold)
}
