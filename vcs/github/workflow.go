package github

const (
	buildAndPushTemplate = `name: "Build Container Image"
on: [ push, pull_request ]

defaults:
  run:
    shell: bash
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  build-docker:
    name: "Build Container image"
    runs-on: ubuntu-latest
    env:
      CONTAINER_IMAGE_NAME: "[[.RegistryEndpoint]]/[[.RepositoryOwner]]/[[.RepositoryName]]:${{ github.sha }}"

    steps:
      - name: Checkout the branch
        uses: actions/checkout@v4
      - name: Setup Docker Buildx
        uses: docker/setup-buildx-action@v3
        id: buildx
        with:
          install: true
          version: latest
      - name: Login to OpenRegistry
        uses: docker/login-action@v3
        with:
          registry: [[.RegistryEndpoint]]
          username: [[.RepositoryOwner]]
          password: ${{ secrets.GITHUB_TOKEN }}
      - name: Build image
        uses: docker/build-push-action@v5
        with:
          context: .
          file: Dockerfile
          platforms: linux/amd64
          push: true
          target: runner
          tags: ${{ env.CONTAINER_IMAGE_NAME }}
`

	InitialPRBody = `
# Description

🤖 🤖  This PR is automatically created by OpenRegistry Github Integration 🤖 🤖 

This pull request includes a **GitHub Actions Workflow** for building and pushing container images to
[OpenRegistry](https://openregistry.dev). 

This PR assumes that _Dockerfile_ is named **Dockerfile** and present inside the root directory of the
project. Please review this pull request and make any necessary changes
(like adding the correct path for _Dockerfile_).

👀 [View your container images]({{ .WebInterfaceURL }})

> **Workflow File Path**: _.github/workflows/openregistry-build-and-push.yml_
`
)

type InitialPRTemplateData struct {
	WebInterfaceURL string
}
