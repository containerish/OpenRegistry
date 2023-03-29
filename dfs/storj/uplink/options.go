package uplink

import (
	"time"

	"github.com/containerish/OpenRegistry/config"
	"storj.io/uplink"
)

func (u *storjUplink) checkAndSetExpiry(opts *uplink.UploadOptions) {
	if u.env == config.Local {
		opts.Expires = time.Now().Add(time.Minute * 30)
	}
}
