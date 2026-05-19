module github.com/ubgo/logger/contrib/sentry

go 1.25.0

replace github.com/ubgo/logger => ../..

require (
	github.com/getsentry/sentry-go v0.46.2
	github.com/ubgo/logger v0.0.0-00010101000000-000000000000
)

require (
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)
