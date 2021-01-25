// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package gtimer

import (
	"time"

	"github.com/gogf/gf/container/glist"
)

// start starts the ticker using a standalone goroutine.
func (w *wheel) start() {
	go func() {
		var (
			tickDuration = time.Duration(w.intervalMs) * time.Millisecond
			ticker       = time.NewTicker(tickDuration)
		)
		for {
			select {
			case <-ticker.C:
				switch w.timer.status.Val() {
				case StatusRunning:
					w.proceed()

				case StatusStopped:
					// Do nothing.

				case StatusClosed:
					ticker.Stop()
					return
				}

			}
		}
	}()
}

// proceed checks and rolls on the job.
// If a timing job is time for running, it runs in an asynchronous goroutine,
// or else it removes from current slot and re-installs the job to another wheel and slot
// according to its leftover interval in milliseconds.
func (w *wheel) proceed() {
	var (
		nowTicks = w.ticks.Add(1)
		list     = w.slots[int(nowTicks%w.number)]
		length   = list.Len()
		nowMs    = w.timer.nowFunc().UnixNano() / 1e6
	)
	if length > 0 {
		go func(l *glist.List, nowTicks int64) {
			var entry *Entry
			for i := length; i > 0; i-- {
				if v := l.PopFront(); v == nil {
					break
				} else {
					entry = v.(*Entry)
				}
				// Checks whether the time for running.
				runnable, addable := entry.check(nowTicks, nowMs)
				if runnable {
					// Just run it in another goroutine.
					go func(entry *Entry) {
						defer func() {
							if err := recover(); err != nil {
								if err != panicExit {
									panic(err)
								} else {
									entry.Close()
								}
							}
							if entry.Status() == StatusRunning {
								entry.SetStatus(StatusReady)
							}
						}()
						entry.job()
					}(entry)
				}
				// Add job again, which make the job continuous running.
				if addable {
					// If StatusReset, reset to runnable state.
					if entry.Status() == StatusReset {
						entry.SetStatus(StatusReady)
					}
					entry.wheel.timer.doAddEntryByParent(!runnable, nowMs, entry.installIntervalMs, entry)
				}
			}
		}(list, nowTicks)
	}
}
