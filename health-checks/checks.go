package healthchecks

import (
	"net/http"
	"time"

	"github.com/alexliesenfeld/health"
	"github.com/containerish/OpenRegistry/store/postgres"
)

func NewHealthChecksAPI(pgPing postgres.PostgresPing) http.HandlerFunc {
	cacheOpt := health.WithCacheDuration(time.Second * 5)
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
