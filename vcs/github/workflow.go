package github

const (
	buildAndPushTemplate = `
name: "Build Container Image"
on:
  workflow_call:
  push:
    branches:
      - [[ . ]]

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
      CONTAINER_IMAGE_NAME: openregistry.dev/${{ github.repository }}:${{ github.sha }}

    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 0

      - name: Setup Docker Buildx
        uses: docker/setup-buildx-action@v2
        id: buildx
        with:
          install: true
          version: latest
	  - name: Login to OpenRegistry
        uses: docker/login-action@v2
        with:
          registry: openregistry.dev
          username: ${{ github.repository_owner }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build image
        uses: docker/build-push-action@v3
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

> This PR is automatically created by OpenRegistry Github Integration

Adds GitHub Actions Workflow for building and pushing container images to OpenRegistry
`
)
