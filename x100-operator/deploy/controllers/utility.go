/*
Copyright (c) 2023 Qualcomm Innovation Center, Inc. All rights reserved.
SPDX-License-Identifier: BSD-3-Clause-Clear
*/

package controllers

import (
        "sigs.k8s.io/controller-runtime/pkg/log"
)

func LogAndExitOnError(logit string, err error) {
     if err != nil {
       if logit != "" {
         log.Log.Info(logit)
       }
       panic(err)
     }
}
