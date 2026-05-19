package logger

import (
	"os"
	"os/signal"
	"syscall"
)

// OnSIGHUP runs fn on every SIGHUP — wire it to RotatingFile.Reopen for
// logrotate compatibility. Returns a stop func.
//
//	stop := logger.OnSIGHUP(func() { _ = rf.Reopen() })
//	defer stop()
func OnSIGHUP(fn func()) (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ch:
				fn()
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
	}
}

// CycleLevelOnSignal toggles lv between verbose and normal on each delivery of
// sig (e.g. syscall.SIGUSR2): first hit → verbose, next → back to normal.
// Lets you flip a stuck prod process to debug without a redeploy or an HTTP
// endpoint. Returns a stop func.
func CycleLevelOnSignal(lv *LevelVar, sig os.Signal, normal, verbose Level) (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sig)
	done := make(chan struct{})
	go func() {
		on := false
		for {
			select {
			case <-ch:
				if on {
					lv.Set(normal)
				} else {
					lv.Set(verbose)
				}
				on = !on
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
	}
}
