#!/bin/bash

set -euo pipefail

docker build -f ./Dockerfile -t "${DISTRIBUTION_REF}" .
docker run --rm -p 5000:5000 \
	--mount="type=bind,source=${PWD}/config.yaml,target=/home/runner/.openregistry/config.yaml" \
	--env="CI_SYS_ADDR=$IP:5000" -d "${DISTRIBUTION_REF}"
sleep 5
curl -XPOST -d ${OPENREGISTRY_SIGNUP_PAYLOAD} "http://${IP}:5000/auth/signup"
