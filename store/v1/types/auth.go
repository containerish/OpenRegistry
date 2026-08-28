package types

import (
	"bytes"
	"strings"
)

type (
	OCITokenPermissonClaim struct {
		Type    string   `json:"type"`
		Name    string   `json:"name"`
		Actions []string `json:"actions"`
	}

	OCITokenPermissonClaimList []*OCITokenPermissonClaim

	Scope struct {
		Actions map[string]bool
		Type    string
		Name    string
	}

	ResetPasswordRequest struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
)

func (c *OCITokenPermissonClaim) HasPushAccess() bool {
	for _, action := range c.Actions {
		if action == "push" {
			return true
		}
	}

	return false
}

func (c *OCITokenPermissonClaim) HasPullAccess() bool {
	for _, action := range c.Actions {
		if action == "pull" {
			return true
		}
	}

	return false
}

func (c *OCITokenPermissonClaim) HasPushAndPullAccess() bool {
	return c.HasPushAccess() && c.HasPullAccess()
}

func (cl OCITokenPermissonClaimList) HasPullAccess(name string) bool {
	for _, claim := range cl {
		if claim.Name == name {
			return claim.HasPullAccess()
		}
	}

	return false
}

func (cl OCITokenPermissonClaimList) MatchUsername(name string) bool {
	for _, claim := range cl {
		nameParts := strings.Split(claim.Name, "/")
		if len(nameParts) == 2 {
			if ok := bytes.EqualFold([]byte(nameParts[0]), []byte(name)); ok {
				return ok
			}
		}
	}

	return false
}

func (cl OCITokenPermissonClaimList) MatchAccount(name string) bool {
	for _, claim := range cl {
		// in this case, name is the username
		if ok := bytes.EqualFold([]byte(claim.Name), []byte(name)); ok {
			return ok
		}
	}

	return false
}

func (cl OCITokenPermissonClaimList) HasPushAccess(name string) bool {
	for _, claim := range cl {
		if claim.Name == name {
			return claim.HasPushAccess()
		}
	}

	return false
}

func (cl OCITokenPermissonClaimList) HasPushAndPullAccess(name string) bool {
	for _, claim := range cl {
		if claim.Name == name {
			return claim.HasPushAndPullAccess()
		}
	}

	return false
}
