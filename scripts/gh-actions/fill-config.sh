#!bin/bash

set -euo pipefail

IP=`hostname -I | awk '{print $1}'`
echo "IP=$IP" >> $GITHUB_ENV
echo "OCI_ROOT_URL=http://$IP:5000" >> $GITHUB_ENV
DISTRIBUTION_REF="local-distribution:v$(date +%Y%m%d%H%M%S)"
cp config.yaml.example config.yaml
yq e -i '.environment = "ci"' config.yaml
IP=$IP yq e -i '.database.host = env(IP)' config.yaml
FILEBASE_KEY=${FILEBASE_KEY} yq e -i '.dfs.s3_any.access_key = env(FILEBASE_KEY)' config.yaml
FILEBASE_SECRET=${FILEBASE_SECRET} yq e -i '.dfs.s3_any.secret_key = env(FILEBASE_SECRET)' config.yaml
FILEBASE_BUCKET=${FILEBASE_BUCKET} yq e -i '.dfs.s3_any.bucket_name = env(FILEBASE_BUCKET)' config.yaml
FILEBASE_ENDPOINT=${FILEBASE_ENDPOINT} yq e -i '.dfs.s3_any.endpoint = env(FILEBASE_ENDPOINT)' config.yaml
FILEBASE_RESOLVER_URL=${FILEBASE_RESOLVER_URL} yq e -i '.dfs.s3_any.dfs_link_resolver = env(FILEBASE_RESOLVER_URL)' config.yaml
