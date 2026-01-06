module github.com/sardanioss/httpcloak

go 1.24.1

require (
	github.com/andybalholm/brotli v1.2.0
	github.com/klauspost/compress v1.18.2
	github.com/miekg/dns v1.1.69
	github.com/sardanioss/http v1.1.0
	github.com/sardanioss/net v1.1.0
	github.com/sardanioss/quic-go v1.2.7
	github.com/sardanioss/utls v1.9.1-0.20260106065245-2e685cf72b97
)

replace github.com/sardanioss/quic-go => ../quic-go

replace github.com/sardanioss/utls => ../utls

require (
	github.com/quic-go/qpack v0.6.0 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/tools v0.39.0 // indirect
)
