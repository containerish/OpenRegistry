package server

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/telemetry"
)

func getTokenFromReq(req connect.AnyRequest, jwtSigningPubKey *rsa.PublicKey) (uuid.UUID, error) {
	token, err := tryTokenFromReqHeaders(req, jwtSigningPubKey)
	if err != nil {
		token, err = tryTokenFromReqCookies(req)
		if err != nil {
			return uuid.Nil, fmt.Errorf("getTokenFromReq: tryTokenFromReqCookies: %w", err)
		}
	}

	return token, nil
}

func tryTokenFromReqCookies(req connect.AnyRequest) (uuid.UUID, error) {
	tmpReq := http.Request{Header: req.Header()}
	sessionCookie, err := tmpReq.Cookie("session_id")
	if err != nil {
		return uuid.Nil, fmt.Errorf("tryTokenFromReqCookies: ERR_NO_COOKIE: %w", err)
	}

	sessionID, err := url.QueryUnescape(sessionCookie.Value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("tryTokenFromReqCookies: ERR_WRONG_ENCODING: %w", err)
	}

	parsedID, err := uuid.Parse(sessionID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("tryTokenFromReqCookies: ERR_UUID_PARSE: %w", err)
	}

	return parsedID, nil
}

func tryTokenFromReqHeaders(req connect.AnyRequest, jwtSigningPubKey *rsa.PublicKey) (uuid.UUID, error) {
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

func getTokenFromConn(
	conn connect.StreamingHandlerConn,
	jwtSigningPubKey *rsa.PublicKey,
	logger telemetry.Logger,
) (uuid.UUID, error) {
	token, err := tryTokenFromConnHeaders(conn, jwtSigningPubKey, logger)
	if err != nil {
		token, err = tryTokenFromConnCookies(conn)
		if err != nil {
			return uuid.Nil, err
		}
	}

	return token, nil
}

func tryTokenFromConnCookies(conn connect.StreamingHandlerConn) (uuid.UUID, error) {
	tmpReq := http.Request{Header: conn.RequestHeader()}
	sessionCookie, err := tmpReq.Cookie("session_id")
	if err != nil {
		return uuid.Nil, err
	}

	sessionID, err := url.QueryUnescape(sessionCookie.Value)
	if err != nil {
		return uuid.Nil, err
	}

	parsedID, err := uuid.Parse(sessionID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("tryTokenFromConnCookies: ERR_UUID_PARSE: %w", err)
	}

	return parsedID, nil
}

func tryTokenFromConnHeaders(
	conn connect.StreamingHandlerConn,
	jwtSigningPubKey *rsa.PublicKey,
	logger telemetry.Logger,
) (uuid.UUID, error) {
	logEvent := logger.Debug().Str("procedure", conn.Spec().Procedure)
	authToken := conn.RequestHeader().Get("Authorization")
	tokenParts := strings.Split(authToken, " ")
	if len(tokenParts) == 2 {
		if !strings.EqualFold(tokenParts[0], "Bearer") {
			errMsg := fmt.Errorf("tryTokenFromConnHeaders: invalid authorization scheme")
			logEvent.Err(errMsg).Send()
			return uuid.Nil, errMsg
		}

		claims := &auth.Claims{}
		token, err := jwt.ParseWithClaims(tokenParts[1], claims, func(t *jwt.Token) (interface{}, error) {
			return jwtSigningPubKey, nil
		})
		if err != nil {
			logEvent.Err(err).Send()
			return uuid.Nil, fmt.Errorf("tryTokenFromConnHeaders: ERR_JWT_CLAIM_PARSE: %w", err)
		}

		if !token.Valid {
			errMsg := fmt.Errorf("tryTokenFromConnHeaders: JWT is invalid")
			logEvent.Err(errMsg).Send()
			return uuid.Nil, errMsg
		}

		claims, ok := token.Claims.(*auth.Claims)
		if !ok {
			errMsg := fmt.Errorf("tryTokenFromConnHeaders: error parsing claims from token")
			logEvent.Err(errMsg).Send()
			return uuid.Nil, errMsg
		}

		parsedID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return uuid.Nil, fmt.Errorf("tryTokenFromConnHeaders: ERR_UUID_PARSE: %w", err)
		}

		logEvent.Bool("success", true).Send()
		return parsedID, nil
	}
	errMsg := fmt.Errorf("tryTokenFromConnHeaders: invalid auth token")
	logEvent.Err(errMsg).Send()
	return uuid.Nil, errMsg
}
