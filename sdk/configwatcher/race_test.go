package configwatcher

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestValue_ConcurrentGetUpdate(t *testing.T) {
	v := newValue(int64(0), parseInt)

	const goroutines = 20
	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func() {
			defer wg.Done()
			for i := range iterations {
				if g%2 == 0 {
					v.Get()
					v.GetWithNull()
				} else {
					v.update(fmt.Sprintf("%d", i), true)
				}
			}
		}()
	}

	wg.Wait()
}

func TestValue_ConcurrentUpdateAndChanges(t *testing.T) {
	v := newValue("default", parseString)

	var wg sync.WaitGroup
	wg.Add(2)

	// Writer goroutine.
	go func() {
		defer wg.Done()
		for i := range 200 {
			v.update(fmt.Sprintf("val-%d", i), true)
		}
		v.close()
	}()

	// Reader goroutine drains the changes channel.
	go func() {
		defer wg.Done()
		for range v.Changes() {
		}
	}()

	wg.Wait()
}

func TestWatcher_ConcurrentRegisterAndPaths(t *testing.T) {
	w := &Watcher{
		fields: make(map[string]*fieldEntry),
		done:   make(chan struct{}),
		opts: options{
			minBackoff: 10 * time.Millisecond,
			maxBackoff: 50 * time.Millisecond,
		},
	}

	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func() {
			defer wg.Done()
			if g%2 == 0 {
				registerField(w, fmt.Sprintf("field-%d", g), int64(0), parseInt)
			} else {
				w.registeredPaths()
			}
		}()
	}

	wg.Wait()
}
