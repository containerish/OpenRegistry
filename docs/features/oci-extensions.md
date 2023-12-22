# OCI Extensions

OCI extensions allows Registry implementers to add additional feature to the registry which can then be proposed
to the OCI management team to be included as features of the Spec. This allows for OpenRegistry to develop new 
registry-specific features and experiment with them. If we discover good use-cases of those features, we'll
send those feature requests upstream to be included in the OCI distribution specification.

Here's the Extensions we're experimenting on right now:
- `CatalogDetail`
  List the list of publicly available repositories. This allows registry clients implement a list repositories feature.
- `RepositoryDetail`
  This extension allows for a client to pull the details of a container image repository. Information like pull count,
  repository owner, stars, size, etc can be then displayed to the end user.
- `ChangeContainerImageVisibility`
  This extension allows a client to toggle the visibility of a container image repository. This would allow for public
  or private repositories as part of the spec and treat them differently
- `GetUserCatalog`
  This extension allows for listing user catalog, which also includes their private repositories. By default catalogs
  only list public repositories
