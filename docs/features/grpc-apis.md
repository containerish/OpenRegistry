# gRPC APIs

OpenRegistry is embracing gRPC APIs as much as possible. This allows for OpenRegistry to use efficient transport and 
data encoding on-the-wire techniques. This also allows for easy client generation and makes the types used across OpenRegistry
unified.

There isn't a specific PR for this since it has been worked over multiple PRs to bring this over.
Some of the PRs are:
- [#524](https://github.com/containerish/OpenRegistry/pull/524)
- [#396](https://github.com/containerish/OpenRegistry/pull/396)
- [#342](https://github.com/containerish/OpenRegistry/pull/342)

Here are the services that have been migrated to gRPC:
- Kon - our container image build automation service
  - GitHub Actions based Build - [GHA Builds V1](https://github.com/containerish/OpenRegistry/blob/main/protos/services/kon/github_actions/v1/build_job.proto)
  - Automation Build Project Service - [Build Project V1](https://github.com/containerish/OpenRegistry/blob/main/protos/services/kon/github_actions/v1/build_project.proto)
- Yor - Container Vulnerability Scanning
  - Clair - [Clair V1](https://github.com/containerish/OpenRegistry/blob/feat/vuln-scanning/protos/services/yor/clair/v1)
