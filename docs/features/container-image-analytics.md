# Container Image Analytics

We store a minimal amount of container image analytics which allows us to show some important metrics about a container
image like how many times a container image has been pulled or how many stars a repository has. We believe these are enough
for a user to determine weather they should trust/engage with a particular repository.

## GitHub Pull Requests:

- Backend - [PR #503](https://github.com/containerish/OpenRegistry/pull/503)
- Frontend - [PR #140](https://github.com/containerish/openregistry-web/pull/140)

## How to test this feature:

Container image pulls are updated automatically behind the scenes every time a container image is pulled by any user on
OpenRegistry. This can be visualised on the OpenRegistry Web app. When you visit your repositories section or the search
page, every container image also shows their pull counter.

As for the repository stars (which tries to mimic GitHub repository stars system), can also be seen on the repository
list item. We have an option to star any repository on OpenRegistry.
