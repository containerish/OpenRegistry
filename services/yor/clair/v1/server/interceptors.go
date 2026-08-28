package server

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/store/v1/types"
)

// NewJWTInterceptor is a UnaryInterceptorFunc that inspects and tries to parse a JWT from the request.
// If the JWT is invalid, an Unauthorized error is returned
func (c *clair) NewJWTInterceptor() connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			logEvent := c.logger.Debug().Str("procedure", req.Spec().Procedure)

			userID, err := c.getTokenFromReq(req, c.authConfig.JWTSigningPubKey)
			if err != nil {
				logEvent.Err(err).Send()
				return nil, err
			}

			user, err := c.userGetter.GetUserByID(ctx, userID)
			if err != nil {
				logEvent.Err(err).Send()
				return nil, connect.NewError(connect.CodeFailedPrecondition, err)
			}

			logEvent.Bool("success", true).Send()
			ctx = context.WithValue(ctx, types.UserContextKey, user)
			return next(ctx, req)
		})
	})
}

func (c *clair) getTokenFromReq(req connect.AnyRequest, jwtSigningPubKey *rsa.PublicKey) (uuid.UUID, error) {
	tokenFromHeaders, headerErr := c.tryTokenFromReqHeaders(req, jwtSigningPubKey)
	if headerErr != nil {
		tokenFromCookies, cookieErr := c.tryTokenFromReqCookies(req, jwtSigningPubKey)
		if cookieErr != nil {
			return uuid.Nil, fmt.Errorf(
				"getTokenFromReq: tryTokenFromReqCookies: %w - tryTokenFromReqHeaders: %w", cookieErr, headerErr,
			)
		}
		return tokenFromCookies, nil
	}

	return tokenFromHeaders, nil
}

func (c *clair) tryTokenFromReqCookies(req connect.AnyRequest, jwtSigningPubKey *rsa.PublicKey) (uuid.UUID, error) {
	tmpReq := http.Request{Header: req.Header()}
	sessionCookie, err := tmpReq.Cookie("access")
	if err != nil {
		return uuid.Nil, fmt.Errorf("tryTokenFromReqCookies: ERR_NO_COOKIE: %w", err)
	}

	authToken, err := url.QueryUnescape(sessionCookie.Value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("tryTokenFromReqCookies: ERR_WRONG_ENCODING: %w", err)
	}

	if authToken != "" {
		claims := &auth.Claims{}
		token, err := jwt.ParseWithClaims(authToken, claims, func(t *jwt.Token) (interface{}, error) {
			return jwtSigningPubKey, nil
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("tryTokenFromReqHeaders: ERR_JWT_CLAIM_PARSE: %w", err)
		}

		claims, ok := token.Claims.(*auth.Claims)
		if !ok {
			return uuid.Nil, fmt.Errorf("tryTokenFromReqHeaders: error parsing claims from token")
		}

		parsedID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return uuid.Nil, fmt.Errorf("tryTokenFromReqHeaders: ERR_UUID_PARSE: %w", err)
		}

		return parsedID, nil
	}

	errMsg := fmt.Errorf("auth token contains invalid parts")
	return uuid.Nil, errMsg
}

func (c *clair) tryTokenFromReqHeaders(req connect.AnyRequest, jwtSigningPubKey *rsa.PublicKey) (uuid.UUID, error) {
	authToken := req.Header().Get("Authorization")
	tokenParts := strings.Split(authToken, " ")
	if len(tokenParts) == 2 {
		if !strings.EqualFold(tokenParts[0], "Bearer") {
			errMsg := fmt.Errorf("tryTokenFromReqHeaders: invalid authorization scheme")
			return uuid.Nil, errMsg
		}

		claims := &auth.Claims{}
		token, err := jwt.ParseWithClaims(tokenParts[1], claims, func(t *jwt.Token) (interface{}, error) {
			return jwtSigningPubKey, nil
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("tryTokenFromReqHeaders: ERR_JWT_CLAIM_PARSE: %w", err)
		}

		claims, ok := token.Claims.(*auth.Claims)
		if !ok {
			return uuid.Nil, fmt.Errorf("tryTokenFromReqHeaders: error parsing claims from token")
		}

		parsedID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return uuid.Nil, fmt.Errorf("tryTokenFromReqHeaders: ERR_UUID_PARSE: %w", err)
		}
		return parsedID, nil
	}

	errMsg := fmt.Errorf("auth token contains invalid parts")
	return uuid.Nil, errMsg
}
