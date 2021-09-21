package contextflight

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestSimpleDoCall(t *testing.T) {
	g := Group{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	res, err, _ := g.Do(ctx, "test", func(ctx context.Context) (interface{}, error) {
		return "foo", ctx.Err()
	})

	require.NoError(t, err)
	assert.Equal(t, "foo", res)
}

func TestSerialDoCallsForSameKey(t *testing.T) {
	g := Group{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	res, err, _ := g.Do(ctx, "test", func(ctx context.Context) (interface{}, error) {
		return "foo", ctx.Err()
	})

	require.NoError(t, err)
	assert.Equal(t, "foo", res)

	res, err, _ = g.Do(ctx, "test", func(ctx context.Context) (interface{}, error) {
		return "foo", ctx.Err()
	})

	require.NoError(t, err)
	assert.Equal(t, "foo", res)
}

func TestTwoDoCalls(t *testing.T) {
	g := Group{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		calls  uint64 = 0
		start         = sync.WaitGroup{}
		finish        = sync.WaitGroup{}
	)

	f := func(ctx context.Context) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		atomic.AddUint64(&calls, 1)

		return "foo", ctx.Err()
	}

	start.Add(1)

	finish.Add(1)
	go func() {
		defer finish.Done()
		start.Wait()
		res, err, _ := g.Do(ctx, "test", f)

		require.NoError(t, err)
		assert.Equal(t, "foo", res)
	}()

	finish.Add(1)
	go func() {
		defer finish.Done()
		start.Wait()
		res, err, _ := g.Do(ctx, "test", f)

		require.NoError(t, err)
		assert.Equal(t, "foo", res)
	}()

	start.Done()
	finish.Wait()

	assert.Equal(t, uint64(1), calls)
}

func TestTwoDoCallsWithOneCancelledContext(t *testing.T) {
	g := Group{}

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	var (
		calls  uint64 = 0
		start         = sync.WaitGroup{}
		finish        = sync.WaitGroup{}
	)

	f := func(ctx context.Context) (interface{}, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		atomic.AddUint64(&calls, 1)

		return "foo", nil
	}

	start.Add(1)

	finish.Add(1)
	go func() {
		defer finish.Done()
		start.Wait()
		res, err, _ := g.Do(ctx1, "test", f)

		require.NoError(t, err)
		assert.Equal(t, "foo", res)
	}()

	finish.Add(1)
	go func() {
		defer finish.Done()
		start.Wait()
		res, err, _ := g.Do(ctx2, "test", f)

		require.NoError(t, err)
		assert.Equal(t, "foo", res)
	}()

	start.Done()
	cancel1()
	finish.Wait()

	assert.Equal(t, uint64(1), calls)
}

func TestTwoDoCallsWithTwoCancelledContext(t *testing.T) {
	g := Group{}

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	var (
		calls  uint64 = 0
		start         = sync.WaitGroup{}
		finish        = sync.WaitGroup{}
	)

	f := func(ctx context.Context) (interface{}, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		atomic.AddUint64(&calls, 1)

		return "foo", nil
	}

	start.Add(1)

	finish.Add(1)
	go func() {
		defer finish.Done()
		start.Wait()
		_, err, _ := g.Do(ctx1, "test", f)

		require.ErrorIs(t, err, context.Canceled)
	}()

	finish.Add(1)
	go func() {
		defer finish.Done()
		start.Wait()
		_, err, _ := g.Do(ctx2, "test", f)

		require.ErrorIs(t, err, context.Canceled)
	}()

	start.Done()
	cancel1()
	cancel2()
	finish.Wait()

	assert.Equal(t, uint64(0), calls)
}
