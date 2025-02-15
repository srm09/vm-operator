// Copyright (c) 2022 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package virtualmachinepublishrequest

import (
	"github.com/pkg/errors"

	ctrlmgr "sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/vmware-tanzu/vm-operator/pkg/context"
	"github.com/vmware-tanzu/vm-operator/webhooks/virtualmachinepublishrequest/validation"
)

func AddToManager(ctx *context.ControllerManagerContext, mgr ctrlmgr.Manager) error {
	if err := validation.AddToManager(ctx, mgr); err != nil {
		return errors.Wrap(err, "failed to initialize validation webhook")
	}
	return nil
}
