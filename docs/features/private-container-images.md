# Private & Encrypted Container Images

Private and encrypted container images are an integral feature of OpenRegistry. Not every user/org wants to keep all of
their container images public. This feature allows your to create new repositories with their visibility mode set to
either `Public` or `Private`. Private images, as their name suggest can only be operated on by the owner of the repository
or member of an organization if the repository is under an organization.
Public repositories can be pulled by any user (and even without being a user on OpenRegistry), but push to such a repository
is restricted to the repository owner or the organization members.

## GitHub Pull Requests:

- Backend - [PR #396](https://github.com/containerish/OpenRegistry/pull/396)
- Frontend - [PR #140](https://github.com/containerish/openregistry-web/pull/140)

## How to test this feature:

All the new container images are `Private` by default. This is a decision we took to ensure no accidental access is
shared to container image. When you push a new container image, if a repository for the container image does not exist,
one will be created for it with it's visibility set to private. 
To create a new repository, visit `https://app.openregistry.dev/repositories` and click the `New Repository` button.
Select the visibility mode, enter name and description fields and create a repository.

You can always toggle the repository's visibility in the repository settings page.
