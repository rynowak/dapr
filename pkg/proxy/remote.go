// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
// ------------------------------------------------------------

package proxy

import "google.golang.org/grpc"

// TODO rationalize duplication with direct messaging.

// messageClientConnection is the function type to connect to the other
// applications to send the message using service invocation.
type messageClientConnection func(address, id string, namespace string, skipTLS, recreateIfExists, enableSSL bool) (*grpc.ClientConn, error)
