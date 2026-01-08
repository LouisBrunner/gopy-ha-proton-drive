module github.com/LouisBrunner/gopy-ha-proton-drive

go 1.24.1

tool honnef.co/go/tools/cmd/staticcheck

replace github.com/henrybear327/Proton-API-Bridge => github.com/LouisBrunner/Proton-API-Bridge v0.0.0-20250414223703-691346f75016

replace github.com/henrybear327/go-proton-api => github.com/LouisBrunner/go-proton-api v0.0.0-20250414221126-b49df39ba33b

require (
	github.com/ProtonMail/gopenpgp/v2 v2.8.2
	github.com/hashicorp/go-version v1.8.0
	github.com/henrybear327/Proton-API-Bridge v1.0.0
	github.com/henrybear327/go-proton-api v1.0.0
	github.com/sirupsen/logrus v1.9.3
	github.com/urfave/cli/v3 v3.2.0
)

require (
	github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c // indirect
	github.com/ProtonMail/bcrypt v0.0.0-20211005172633-e235017c1baf // indirect
	github.com/ProtonMail/gluon v0.17.1-0.20230724134000-308be39be96e // indirect
	github.com/ProtonMail/go-crypto v1.1.5 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/ProtonMail/go-srp v0.0.7 // indirect
	github.com/PuerkitoBio/goquery v1.8.1 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/bradenaw/juniper v0.15.3 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/cronokirby/saferith v0.33.0 // indirect
	github.com/emersion/go-message v0.18.2 // indirect
	github.com/emersion/go-vcard v0.0.0-20241024213814-c9703dde27ff // indirect
	github.com/go-resty/resty/v2 v2.16.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/relvacode/iso8601 v1.6.0 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	golang.org/x/tools/go/expect v0.1.1-deprecated // indirect
	honnef.co/go/tools v0.6.1 // indirect
)
