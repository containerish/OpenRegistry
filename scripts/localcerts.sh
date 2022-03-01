#!/bin/bash

DOMAIN="openregistry.local"
CERTS_DIR=".certs"
mkdir -p ${CERTS_DIR}
openssl req -newkey rsa:2048 -nodes -keyout ${CERTS_DIR}/${DOMAIN}.key -x509 -days 365 -out ${CERTS_DIR}/${DOMAIN}.crt -subj \
"/C=US/ST=Oregon/L=Portland/O=Company Name/OU=Org/CN=${DOMAIN}"

