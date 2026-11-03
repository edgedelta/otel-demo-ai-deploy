# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

FROM docker.io/library/python:3.15.0-slim-bookworm@sha256:0e019bbdfb5d9874cb2389f1c2884b64aa2ac97c99960b5bc521cef978c0617d

WORKDIR /build

COPY ./src/recommendation/genproto/requirements.txt requirements.txt

RUN python -m pip install --no-cache-dir --require-hashes -r requirements.txt
