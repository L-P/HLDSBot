package hlds_test

import (
	"hldsbot/hlds"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPortAlloc(t *testing.T) {
	var min uint16 = 27015
	pool, err := hlds.NewPool(nil, 2, 27015)
	require.NoError(t, err)

	port, err := pool.AllocPort()
	require.NoError(t, err, "port allocated")
	require.GreaterOrEqual(t, port, min, "port allocated within bounds")

	port2, err := pool.AllocPort()
	require.NoError(t, err, "port allocated")
	require.GreaterOrEqual(t, port, min, "port allocated within bounds")
	require.NotEqual(t, port, port2, "ports are not allocated twice")

	_, err = pool.AllocPort()
	require.Error(t, err, "cannot allocate more ports than given capacity")

	pool.FreePort(port)
	port3, err := pool.AllocPort()
	require.NoError(t, err, "port allocated")
	require.GreaterOrEqual(t, port, min, "port allocated within bounds")
	require.Equal(t, port, port3, "re-alloacted free'd port")
}
