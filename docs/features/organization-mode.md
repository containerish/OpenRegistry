# Organization Mode

Organization mode allows you to convert your existing account to an organization and manage the access to your container
images for users in your organization. This is a great feature for teams as it allows you to replicate your internal 
hierarchy in OpenRegistry. You can add users to your organization or remove them. Configure permissions for them, 
allowing or denying them access to your organization resources.

## GitHub Pull Requests:

- Backend - [PR #491](https://github.com/containerish/OpenRegistry/pull/491)

## How to test this feature:

This is an account level feature. You can only convert a user account to an organization and the reverse is not allowed.
To convert a user account to an organization, navigate to `Settings -> Organization Mode`.

Here, you'll see an option to convert your account to an org. Once an account is converted to an organization, you'll
have the option to add as many users to your organization. This feature is quite simple but please keep in mind that
when you're adding a user to your organization, you're essentially sharing your container images with them. This means
they can Push, Pull or manage your container images,

> [!NOTE]
> As expected, a user of your org can push or pull your private container images.

> [!WARNING]
> Delete repository isn't allowed at the moment on client side but will be in a future release
