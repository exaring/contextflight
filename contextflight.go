package contextflight

import (
	"context"
	"sync"

	"golang.org/x/sync/singleflight"
)

// Group represents a class of work and forms a namespace in which units of work can be executed with duplicate
// suppression.
type Group struct {
	once  sync.Once
	kcs   map[string]*keyContext
	kcsMu sync.Mutex
	sf    singleflight.Group
}

// Result holds the results of Do, so they can be passed on a channel.
type Result singleflight.Result

type keyContext struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel func()
}

// Do executes and returns the results of the given function, making sure that only one execution is in-flight for a
// given key at a time. If a duplicate comes in, the duplicate caller waits for the original to complete and receives
// the same results. The return value shared indicates whether v was given to multiple callers.
// The context passed to the given function will be cancelled iff all callers' contexts are cancelled.
func (g *Group) Do(ctx context.Context, key string, fn func(context.Context) (interface{}, error)) (v interface{}, err error, shared bool) {
	g.once.Do(func() {
		g.kcs = make(map[string]*keyContext)
	})

	kc := g.getKeyContext(key)

	kc.wg.Add(1)

	// This is the caller's context, which we need to cancel when we are done to avoid leaking the goroutine below.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		kc.wg.Done()
	}()

	return g.sf.Do(key, func() (interface{}, error) {
		defer kc.cancel()
		defer g.deleteKeyContext(key)

		return fn(kc.ctx)
	})
}

// DoChan is like Do but returns a channel that will receive the results when they are ready.
//
// The returned channel will not be closed.
func (g *Group) DoChan(ctx context.Context, key string, fn func(context.Context) (interface{}, error)) <-chan Result {
	g.once.Do(func() {
		g.kcs = make(map[string]*keyContext)
	})

	kc := g.getKeyContext(key)

	kc.wg.Add(1)

	// This is the caller's context, which we need to cancel when we are done to avoid leaking the goroutine below.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		kc.wg.Done()
	}()

	return convertResultChan(g.sf.DoChan(key, func() (interface{}, error) {
		defer kc.cancel()
		defer g.deleteKeyContext(key)

		return fn(kc.ctx)
	}))
}

// Forget tells the singleflight to forget about a key. Future calls to Do for this key will call the function rather
// than waiting for an earlier call to complete.
func (g *Group) Forget(key string) {
	g.sf.Forget(key)
}

// getKeyContext returns a key-specific context
func (g *Group) getKeyContext(key string) *keyContext {
	g.kcsMu.Lock()
	defer g.kcsMu.Unlock()

	kc := g.kcs[key]
	if kc == nil {
		kc = &keyContext{}

		// This is the context passed to the singleflight function.
		ctx, cancel := context.WithCancel(context.Background())
		kc.ctx = ctx
		kc.cancel = cancel
		kc.wg = sync.WaitGroup{}

		go func() {
			kc.wg.Wait()
			kc.cancel()
		}()

		g.kcs[key] = kc
	}

	return kc
}

// deleteKeyContext removes the context identified by key from the group.
func (g *Group) deleteKeyContext(key string) {
	g.kcsMu.Lock()
	defer g.kcsMu.Unlock()

	delete(g.kcs, key)
}

func convertResultChan(in <-chan singleflight.Result) <-chan Result {
	out := make(chan Result, 1)

	go func() {
		for r := range in {
			out <- Result(r)
		}
	}()

	return out
}
