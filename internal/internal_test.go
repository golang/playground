// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"context"
	"testing"
	"time"
)

func TestPeriodicallyDo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	didWork := make(chan time.Time, 2)
	done := make(chan interface{})
	go func() {
		PeriodicallyDo(ctx, 100*time.Millisecond, func(ctx context.Context, t time.Time) {
			select {
			case didWork <- t:
			default:
				// No need to assert that we can't send, we just care that we sent.
			}
		})
		close(done)
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Error("PeriodicallyDo() never called f, wanted at least one call")
	case <-didWork:
		// PeriodicallyDo called f successfully.
	}

	select {
	case <-done:
		t.Errorf("PeriodicallyDo() finished early, wanted it to still be looping")
	case <-didWork:
		cancel()
	}

	select {
	case <-time.After(time.Second):
		t.Fatal("PeriodicallyDo() never returned, wanted return after context cancellation")
	case <-done:
		// PeriodicallyDo successfully returned.
	}
}
