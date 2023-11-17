package healthchecks

import (
	"net/http"
	"time"

	"github.com/alexliesenfeld/health"
	store_v2 "github.com/containerish/OpenRegistry/store/v1"
)

func NewHealthChecksAPI(pgPing store_v2.PostgresPing) http.HandlerFunc {
	cacheOpt := health.WithCacheDuration(time.Second * 30)
	timeoutOpt := health.WithTimeout(time.Second * 10)
	dbHealthOpt := health.WithCheck(health.Check{
		Name:               "database",
		Check:              pgPing.Ping,
		Timeout:            time.Second * 5,
		MaxContiguousFails: 3,
	})

	checker := health.NewChecker(
		cacheOpt,
		timeoutOpt,
		dbHealthOpt,
	)

	return health.NewHandler(checker)
}
