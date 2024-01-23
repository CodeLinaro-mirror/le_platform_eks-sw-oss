#Copyright (c) 2023 Qualcomm Innovation Center, Inc. All rights reserved.
#SPDX-License-Identifier: BSD-3-Clause-Clear

#Copyright 2023.

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


# Build the x100-operator binary
FROM golang:1.21.0 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
RUN go env -w GOTOOLCHAIN=local
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o x100-operator main.go


FROM public.ecr.aws/docker/library/ubuntu:22.04
WORKDIR /
COPY --from=builder /workspace/x100-operator /usr/bin

RUN mkdir -p /opt/x100-operator
COPY assets /opt/x100-operator/

# Add the user account to run as user
USER 65532:65532


ENTRYPOINT ["/usr/bin/x100-operator"]

