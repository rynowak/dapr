// ------------------------------------------------------------
// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.
// ------------------------------------------------------------

package proxy

import (
	"strings"

	"github.com/pkg/errors"
)

// TODO - unify this with the version in direct messaging
type resolvedAppID struct {
	Original  string
	Namespace string
	AppID     string
}

// requestAppIDAndNamespace takes an app id and returns the app id, namespace and error.
func resolveTargetAppID(namespace string, original string) (resolvedAppID, error) {
	items := strings.Split(original, ".")
	if len(items) == 1 {
		return resolvedAppID{original, namespace, original}, nil
	} else if len(items) == 2 {
		return resolvedAppID{original, items[1], items[0]}, nil
	} else {
		return resolvedAppID{}, errors.Errorf("invalid app id %s", original)
	}
}
