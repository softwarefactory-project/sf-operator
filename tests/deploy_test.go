// Copyright (C) 2026 Red Hat
// SPDX-License-Identifier: Apache-2.0

package sf_test

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Deploy Tests", Ordered, func() {
	It("Reconcile default", func() {
		runReconcile(sf)
	})
})
