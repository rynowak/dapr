// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
// ------------------------------------------------------------

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"

	nr "github.com/dapr/components-contrib/nameresolution"
	daprhttp "github.com/dapr/dapr/pkg/http"
	"github.com/dapr/dapr/pkg/logger"
	internalv1pb "github.com/dapr/dapr/pkg/proto/internals/v1"
)

var proxyLogger = logger.NewLogger("dapr.runtime.proxy")

// Server is an interface for the dapr proxy server.
type Server interface {
	StartNonBlocking() error
}

// ServerConfig is the configuration of the server.
type ServerConfig struct {
	ProxyPort       int
	ApplicationPort int
	Namespace       string
	AppID           string
}

type server struct {
	config              ServerConfig
	monitor             StatusMonitor
	resolver            nr.Resolver
	logger              logger.Logger
	connectionCreatorFn messageClientConnection
}

// NewProxyServer creates and returns a new server.
func NewProxyServer(config ServerConfig, monitor StatusMonitor, resolver nr.Resolver, connectionCreatorFn messageClientConnection) Server {
	return &server{config: config, monitor: monitor, resolver: resolver, connectionCreatorFn: connectionCreatorFn}
}

// StartNonBlocking starts the server in a goroutine.
func (s *server) StartNonBlocking() error {
	proxy := httputil.ReverseProxy{
		Director:  s.director,
		Transport: s,
	}

	go func() {
		// Close enough for jazz....
		s.monitor.SetStatus(ServerStatus{Enabled: true, Port: s.config.ProxyPort})

		err := http.ListenAndServe(fmt.Sprintf(":%v", s.config.ProxyPort), &proxy)
		if err != nil {
			s.logger.Fatalf("proxy serve error: %v", err)
		}
	}()
	return nil
}

// Modify the request before sending it on - we have to set the scheme and hostname here.
// However, we can't respond with an error :(
func (s *server) director(req *http.Request) {
}

// Perform the actual transport of the request
func (s *server) RoundTrip(req *http.Request) (*http.Response, error) {
	appIDHeader, ok := req.Header["Destination-App-Id"]
	if !ok || len(appIDHeader) != 1 || appIDHeader[0] == "" {
		msg := daprhttp.NewErrorResponse("ERR_MISSING_APPID", "the appid must be specified using the Destination-App-Id header")
		s.logger.Debugf("request did not contain Destination-App-Id header")
		return respondWithError(400, msg), nil
	}

	appID, err := resolveTargetAppID(s.config.Namespace, appIDHeader[0])
	if err != nil {
		msg := daprhttp.NewErrorResponse("ERR_MISSING_APPID", fmt.Sprintf("the appid %v is invalid", appIDHeader[0]))
		s.logger.Debugf("request contained invalid Destination-App-Id header: %v", appIDHeader[0])
		return respondWithError(400, msg), nil
	}

	if appID.AppID == s.config.AppID && appID.Namespace == s.config.Namespace {
		return s.roundTripLocal(req)
	}

	return s.roundTripRemote(req, appID)
}

func (s *server) roundTripLocal(req *http.Request) (*http.Response, error) {
	// HAXXX the docs say not to do this. But I have problems with authority.
	// Literally problems with authority because I'm changing the authority section of the URL
	// #URLJOKES
	//
	// The right fix here is to spread this logic between `director` and here.
	// the problem is that a `director` can't return errors :(.
	req.URL.Scheme = "http"
	req.URL.Host = fmt.Sprintf("localhost:%d", s.config.ApplicationPort) // TODO cache this

	// The default transport will cache connections for us
	return http.DefaultTransport.RoundTrip(req)
}

func (s *server) roundTripRemote(req *http.Request, appID resolvedAppID) (*http.Response, error) {
	rreq := nr.ResolveRequest{ID: appID.AppID, Namespace: appID.Namespace, Port: s.config.ProxyPort}
	addr, err := s.resolver.ResolveID(rreq)
	if err != nil {
		msg := daprhttp.NewErrorResponse("ERR_UNRESOLVED_APPID", fmt.Sprintf("the appid %v cannot be resolved to a destination", appID.Original))
		s.logger.Debugf("request destination app-id could not be resolved: %v", appID.Original)
		return respondWithError(400, msg), nil
	}

	// addr will be address of the remote GRPC endpoint - we need to call through it to get the port info.
	addr, err = s.getRemoteProxyAddress(addr, appID.AppID, appID.Namespace)
	if err != nil {
		msg := daprhttp.NewErrorResponse("ERR_INTERNAL", "DERP")
		s.logger.Debug("DERP")
		return respondWithError(500, msg), nil
	}

	// HAXXX ^^^
	//
	// This isn't the right approach, because it puts a data-plane operation between Dapr sidecars
	// on the hot path for user traffic. This should be part of the resolver so that it can be cached
	// and represented along with other address concerns. However that requires me to update
	// components-contrib in tandem, so I'm keeping the bad approach while this is a proof of concept.

	// HAXXX the docs say not to do this. But I have problems with authority.
	// Literally problems with authority because I'm changing the authority section of the URL
	// #URLJOKES
	//
	// The right fix here is to spread this logic between `director` and here.
	// the problem is that a `director` can't return errors :(.
	req.URL.Scheme = "http"
	req.URL.Host = addr

	// The default transport will cache connections for us
	return http.DefaultTransport.RoundTrip(req)
}

func respondWithError(code int, e daprhttp.ErrorResponse) *http.Response {
	b, _ := json.Marshal(&e)
	res := http.Response{
		StatusCode: 400,
		Header: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Body: ioutil.NopCloser(bytes.NewReader(b)),
	}
	return &res
}

func (s *server) getRemoteProxyAddress(addr string, appID string, namespace string) (string, error) {
	conn, err := s.connectionCreatorFn(addr, appID, namespace, false, false, false)
	if err != nil {
		return "", err
	}

	c := internalv1pb.NewProxyClient(conn)

	req := internalv1pb.ProxyStatusRequest{Ver: internalv1pb.APIVersion_V1}
	status, err := c.GetProxyStatus(context.Background(), &req)
	if err != nil {
		return "", err
	}

	if !status.Enabled {
		return "", errors.New("remote proxy is not enabled")
	}

	// HAXXX
	parts := strings.Split(addr, ":")
	parts[len(parts)-1] = fmt.Sprintf("%d", status.Port)
	combined := strings.Join(parts, ":")
	return combined, nil
}
