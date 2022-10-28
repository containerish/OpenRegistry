## Contribution Guidelines

Thank you for investing your time in OpenRegistry, your contributions are priceless to us.
There is a lot to contribute to in OpenRegistry and we have opened issues for mostly all of them.
You can start by solving an existing issue or creating a new one.
To make the most out of your efforts and ours, please follow the guidelines below:

1. Within your fork of [OpenRegistry](https://github.com/containerish/OpenRegistry) create a branch for your contribution. Use a meaningful name.
2. Create your contribution, meeting all [contribution quality standards](#contribution-quality-standards)
3. Create a pull request against the main branch of the OpenRegistry repository. Best practice is to follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)
4. Add two reviewers to your pull request (a maintainer will do that for you if you're new). Work with your reviewers to address any comments and obtain a minimum of 2 approvals, at least one of which must be provided by [a maintainer](https://github.com/containerish/OpenRegistry/blob/main/MAINTAINERS.md). To update your pull request amend existing commits whenever applicable and then push the new changes to your pull request branch.
5. Once the pull request is approved, one of the maintainers will merge it.

### Contribution Quality Standards

Your contribution needs to meet the following standards:

- Separate each logical change into its own commit.
- Each commit must pass all code style tests, and the full pull request must pass all integration tests.
- Add a descriptive message for each commit. Follow commit message best practices.
- The commit title must contain a [Commit type](#commit-type-terminology) followed by a short title and description
- A good commit message may look like:

```
Commit type: A descriptive title of 50 characters or fewer.

A concise description where each line is 72 characters or fewer.

Signed-off-by: <A full name> <A email>
Co-authored-by: <B full name> <B email>
```

- Always make sure to sign your commits
- When the Proposed change is a breaking change i.e There's a feature which can cause a large refactor/rewrite or causes us to change other functionality, then we add the ```!``` mark with the commit type, declaring a breaking change, e.g:

```
chore!: drop support for Node 6

Use JavaScript features not available in Node 6.

Signed-off-by: A
```

- Document your pull requests. Include the reasoning behind each change, and the testing done.
- When raising an issue, follow the issue template in place to help us understand the issue in detail

### Commit Type Terminology

| First Word | Meaning                                                                                                |
| ---------- | ------------------------------------------------------------------------------------------------------ |
| feat       | Add a Feature                                                                                          |
| add        | Create a capability                                                                                    |
| fix        | A bug fix                                                                                              |
| doc        | updates to documentation such as a the README or other markdown files                                  |
| perf       | A code change that improves performance                                                                |
| refactor   | Code refactor                                                                                          |
| change     | Change an existing funtionality e.g. feature, test, dependency                                         |
| remove     | Remove a capability                                                                                    |
| style      | Changes that do not affect the meaning of the code (white-space, formatting, missing semi-colons, etc) |
| test       | Adding missing tests or correcting existing tests                                                      |
| chore      | work that isn't directly connected to the product                                                      |

Commit type must never contain (and / or start with) anything else. Especially not something that's unique to your system

Considering every bit of information above, here's a good and a bad commit message:

**Good**:
```
feat: github app integration

With this feature implemented, we will be able to set up an automatic ci with user's github and trigger automatic builds and deployments for them

Signed-off-by: guacamole
```
**Bad**:
```
fixed bug on landing page
Changed style
oops
I think I fixed it this time?
```
