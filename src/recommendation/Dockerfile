# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0


FROM docker.io/library/python:3.15.0-alpine3.24@sha256:2699b90f4d562a22e2dd076c82f964032f64814204bff36d271a54ba4cc2c7e2 AS build-venv

RUN apk update && \
    apk add gcc g++ linux-headers

COPY ./src/recommendation/requirements.txt requirements.txt

RUN python -m venv venv && \
    venv/bin/pip install --no-cache-dir --require-hashes -r requirements.txt

RUN venv/bin/opentelemetry-bootstrap -a install

FROM docker.io/library/python:3.15.0-alpine3.24@sha256:2699b90f4d562a22e2dd076c82f964032f64814204bff36d271a54ba4cc2c7e2

COPY --from=build-venv /venv/ /venv/

WORKDIR /app

COPY ./src/recommendation/demo_pb2_grpc.py demo_pb2_grpc.py
COPY ./src/recommendation/demo_pb2.py demo_pb2.py
COPY ./src/recommendation/logger.py logger.py
COPY ./src/recommendation/metrics.py metrics.py
COPY ./src/recommendation/recommendation_server.py recommendation_server.py

ARG RECOMMENDATION_PORT=9001
EXPOSE ${RECOMMENDATION_PORT}
ENTRYPOINT [ "/venv/bin/opentelemetry-instrument", "/venv/bin/python", "recommendation_server.py" ]
