package apiserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDispatchTracker_Cooldown(t *testing.T) {
	tr := newDispatchTracker(time.Minute)
	now := time.Now()

	allowed, _ := tr.allow("a", now)
	require.True(t, allowed)

	allowed, remaining := tr.allow("a", now.Add(30*time.Second))
	require.False(t, allowed)
	require.InDelta(t, float64(30*time.Second), float64(remaining), float64(time.Second))

	allowed, _ = tr.allow("a", now.Add(2*time.Minute))
	require.True(t, allowed)
}

func TestDispatchTracker_KeysIndependent(t *testing.T) {
	tr := newDispatchTracker(time.Minute)
	now := time.Now()

	allowed, _ := tr.allow("a", now)
	require.True(t, allowed)

	allowed, _ = tr.allow("b", now)
	require.True(t, allowed)
}

func TestDispatchTracker_Forget(t *testing.T) {
	tr := newDispatchTracker(time.Minute)
	now := time.Now()

	allowed, _ := tr.allow("a", now)
	require.True(t, allowed)

	tr.forget("a")

	allowed, _ = tr.allow("a", now)
	require.True(t, allowed)
}
