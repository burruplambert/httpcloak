module github.com/sardanioss/httpcloak

go 1.24.1

require (
	github.com/andybalholm/brotli v1.2.0
	github.com/klauspost/compress v1.18.2
	github.com/miekg/dns v1.1.69
	github.com/sardanioss/http v0.1.0
	github.com/sardanioss/net v0.1.0
	github.com/sardanioss/quic-go v0.1.0
	github.com/sardanioss/utls v0.1.0
)

require (
	github.com/bdandy/go-errors v1.2.2 // indirect
	github.com/bdandy/go-socks4 v1.2.3 // indirect
	github.com/bogdanfinn/fhttp v0.6.4 // indirect
	github.com/bogdanfinn/quic-go-utls v1.0.5-utls // indirect
	github.com/bogdanfinn/tls-client v1.12.0 // indirect
	github.com/bogdanfinn/utls v1.7.5-barnius // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/tam7t/hpkp v0.0.0-20160821193359-2b70b4024ed5 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/tools v0.39.0 // indirect
)

replace github.com/sardanioss/http => ./temp/sardanioss-http

replace github.com/sardanioss/net => ./temp/sardanioss-net

replace github.com/sardanioss/utls => ./temp/sardanioss-utls

replace github.com/sardanioss/quic-go => ./temp/sardanioss-quic-go
