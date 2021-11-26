#!/bin/bash

# This script will build and run OpenRegistry, then build and run
# the OCI conformance tests against it.
# It depends on bash, docker and git being available.

set -eu

spec_name=distribution-spec
spec_version=v1.0.1
prod_name=example
trow_version=v0.2.0
OPENREGISTRY_IMAGE_NAME="openregistry-distribution-build:v$(date +%Y%m%d%H%M%S)"

# check out to OpenRegistry repo
rm -rf OpenRegistry && git clone git@github.com:containerish/OpenRegistry.git
pushd OpenRegistry
docker build -f ./Dockerfile -t "${OPENREGISTRY_IMAGE_NAME}" .

IP=$(ifconfig | grep 192 | awk '{print $2}')
sed -in "s/OPEN_REGISTRY_ENVIRONMENT=local/OPEN_REGISTRY_ENVIRONMENT=ci/g" env-vars.example
sed -in "s/OCI_ROOT_URL=http:\/\/0.0.0.0:5000/OCI_ROOT_URL=http:\/\/$IP:5000/g" conformance.vars

##IP=`hostname -I | awk '{print $1}'`
echo CI_SYS_ADDR="$IP":5000 >> env-vars.example

docker run --rm -p 5000:5000 --name openregistry -d --env-file=env-vars.example "${OPENREGISTRY_IMAGE_NAME}"
sleep 5
curl -XPOST -d '{"email":"johndoe@example.com","username":"johndoe","password":"Qwerty@123"}' "http://0.0.0.0:5000/auth/signup"
popd
# check out conformance repo
rm -rf conf-tmp && git clone https://github.com/opencontainers/${spec_name}.git conf-tmp
pushd conf-tmp/conformance && docker build -t conformance:latest -f Dockerfile .
popd


docker run --rm \
  -v $(pwd)/results:/results \
  -w /results \
  --env-file=OpenRegistry/conformance.vars \
  conformance:latest

cp results/report.html ./
cp results/junit.xml ./
rm -rf conf-tmp
docker rm -f openregistry
