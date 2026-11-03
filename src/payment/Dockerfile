# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


FROM ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-nodejs:0.82.0-3@sha256:609216a9e468279bd41b56829c1fd5e73cb29b5b1002ed95921bf59da854e09a AS otelagent

# -----------------------------------------------------------------------------

FROM docker.io/library/node:26.13.0-slim@sha256:e4db4c914bf8ad241bd24804fc25d272d89bad97e68cdc8a1414862790a02c39 AS builder

WORKDIR /usr/src/app/

COPY ./src/payment/package.json package.json
COPY ./src/payment/package-lock.json package-lock.json

RUN npm ci --omit=dev

# -----------------------------------------------------------------------------

FROM gcr.io/distroless/nodejs22-debian12:nonroot@sha256:13593b7570658e8477de39e2f4a1dd25db2f836d68a0ba771251572d23bb4f8e

WORKDIR /usr/src/app/

COPY --from=builder /usr/src/app/node_modules/ node_modules/

# Path and NODE_OPTIONS value match what the OpenTelemetry Operator injects, so a
# statically built image and an operator-injected pod behave identically. Changing
# either breaks that equivalence.
COPY --from=otelagent /autoinstrumentation /otel-auto-instrumentation-nodejs

COPY ./pb/demo.proto demo.proto

COPY ./src/payment/charge.js charge.js
COPY ./src/payment/index.js index.js
COPY ./src/payment/logger.js logger.js

ARG PAYMENT_PORT=50051
EXPOSE ${PAYMENT_PORT}

ENV NODE_OPTIONS="--require /otel-auto-instrumentation-nodejs/autoinstrumentation.js"

CMD ["index.js"]
