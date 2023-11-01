package config

// There are config constants
const (
	SkipSPV           = true
	SubscriptionID    = "5af4235fe3e2a36965a46805a10dd48e0d659467c7f5df0a8c48ba5d32e406dd"
	MinerAPIEndpoint  = "https://mapi.gorillapool.iom/mapi/tx/"
	JunglebusEndpoint = "https://germany5.gorillapool.io/" // "https://hetzner.junglebus.gorillapool.io/" // "https://germany.junglebus.gorillapool.io/"
	FromBlock         = 574287                             // "Welcome to the Future" post = 574287
	BockSyncRetries   = 5                                  // number of retries before block is marked failed
	DeleteAfterIngest = true                               // delete json data files after ingesting to db
)
