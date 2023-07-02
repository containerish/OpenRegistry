package server

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/bufbuild/connect-go"
	"github.com/containerish/OpenRegistry/auth"
	"github.com/containerish/OpenRegistry/telemetry"
	"github.com/fatih/color"
	"github.com/golang-jwt/jwt/v5"
)

func getTokenFromReq(req connect.AnyRequest, jwtSigningPubKey *rsa.PublicKey) (string, error) {
	token, err := tryTokenFromReqCookies(req)
	if err != nil {
		token, err = tryTokenFromReqHeaders(req, jwtSigningPubKey)
		if err != nil {
			return "", err
		}
	}

	return token, nil
}

func tryTokenFromReqCookies(req connect.AnyRequest) (string, error) {
	tmpReq := http.Request{Header: req.Header()}
	sessionCookie, err := tmpReq.Cookie("session_id")
	if err != nil {
		return "", err
	}

	sessionID, err := url.QueryUnescape(sessionCookie.Value)
	if err != nil {
		return "", err
	}

	return sessionID, nil
}

func tryTokenFromReqHeaders(req connect.AnyRequest, jwtSigningPubKey *rsa.PublicKey) (string, error) {
	authToken := req.Header().Get("Authorization")
	tokenParts := strings.Split(authToken, " ")
	if len(tokenParts) == 2 {
		if !strings.EqualFold(tokenParts[0], "Bearer") {
			errMsg := fmt.Errorf("invalid authorization scheme")
			return "", errMsg
		}

		claims := &auth.Claims{}
		token, err := jwt.ParseWithClaims(tokenParts[1], claims, func(t *jwt.Token) (interface{}, error) {
			return jwtSigningPubKey, nil
		})
		if err != nil {
			return "", err
		}

		claims, ok := token.Claims.(*auth.Claims)
		if !ok {
			return "", fmt.Errorf("error parsing claims from token")
		}

		return claims.Subject, nil
	}

	errMsg := fmt.Errorf("auth token contains invalid parts")
	return "", errMsg
}

func getTokenFromConn(
	conn connect.StreamingHandlerConn,
	jwtSigningPubKey *rsa.PublicKey,
	logger telemetry.Logger,
) (string, error) {
	token, err := tryTokenFromConnHeaders(conn, jwtSigningPubKey, logger)
	if err != nil {
		token, err = tryTokenFromConnCookies(conn)
		if err != nil {
			return "", err
		}
	}

	return token, nil
}

func tryTokenFromConnCookies(conn connect.StreamingHandlerConn) (string, error) {
	tmpReq := http.Request{Header: conn.RequestHeader()}
	sessionCookie, err := tmpReq.Cookie("session_id")
	if err != nil {
		return "", err
	}

	sessionID, err := url.QueryUnescape(sessionCookie.Value)
	if err != nil {
		return "", err
	}

	return sessionID, nil
}

func tryTokenFromConnHeaders(
	conn connect.StreamingHandlerConn,
	jwtSigningPubKey *rsa.PublicKey,
	logger telemetry.Logger,
) (string, error) {
	logEvent := logger.Debug().Str("procedure", conn.Spec().Procedure)
	authToken := conn.RequestHeader().Get("Authorization")
	tokenParts := strings.Split(authToken, " ")
	if len(tokenParts) == 2 {
		if !strings.EqualFold(tokenParts[0], "Bearer") {
			errMsg := fmt.Errorf("invalid authorization scheme")
			logEvent.Err(errMsg).Send()
			return "", errMsg
		}

		claims := &auth.Claims{}
		token, err := jwt.ParseWithClaims(tokenParts[1], claims, func(t *jwt.Token) (interface{}, error) {
			return jwtSigningPubKey, nil
		})
		if err != nil {
			logEvent.Err(err).Send()
			return "", err
		}

		if !token.Valid {
			errMsg := fmt.Errorf("JWT is invalid")
			logEvent.Err(errMsg).Send()
			return "", errMsg
		}

		claims, ok := token.Claims.(*auth.Claims)
		if !ok {
			errMsg := fmt.Errorf("error parsing claims from token")
			logEvent.Err(errMsg).Send()
			return "", errMsg
		}

		color.Yellow("claims from token: %#v", claims.Subject)
		logEvent.Bool("success", true).Send()
		return claims.Subject, nil
	}
	errMsg := fmt.Errorf("invalid auth token")
	logEvent.Err(errMsg).Send()
	return "", errMsg
}
