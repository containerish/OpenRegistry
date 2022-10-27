
## Contribution Guidelines

Thank you for investing your time in OpenRegistry, your contributions are priceless to us.
There is a lot to contribute to in OpenRegistry and we have opened issues for mostly all of them.
You can start by solving an existing issue or creating a new one.
To make the most out of your efforts and ours, please follow the guidelines below:

1. Within your fork of [OpenRegistry](https://github.com/containerish/OpenRegistry) create a branch for your contribution. Use a meaningful name.
2. Create your contribution, meeting all [contribution quality standards](#contribution-quality-standards)
3. Create a pull request against the main branch of the OpenRegistry repository. Make sure to follow the Pull request template in place
4. Add two reviewers to your pull request (a maintainer will do that for you if you're new). Work with your reviewers to address any comments and obtain a minimum of 2 approvals, at least one of which must be provided by [a maintainer](https://github.com/containerish/OpenRegistry/blob/main/MAINTAINERS.md). To update your pull request amend existing commits whenever applicable and then push the new changes to your pull request branch.
5. Once the pull request is approved, one of the maintainers will merge it.

### Contribution Quality Standards

Your contribution needs to meet the following standards:
- Separate each logical change into its own commit.
- Each commit must pass all code style tests, and the full pull request must pass all integration tests. 
- Add a descriptive message for each commit. Follow commit message best practices.
- The commit title must contain a [Subject line](#subject-line-standard-terminology)
- A good commit message may look like:
```
Subject line: A descriptive title of 50 characters or fewer.
A concise description where each line is 72 characters or fewer.
 
Signed-off-by: <A full name> <A email>
Co-authored-by: <B full name> <B email>

```
- Always make sure to sign your commits
- Document your pull requests. Include the reasoning behind each change, and the testing done.
- When raising an issue, follow the issue template in place to help us understand the issue at it's very core

### Subject Line Standard Terminology

First Word | Meaning
--- | --
Add | Create a capability e.g. feature, test, dependency.
Cut | Remove a capability e.g. feature, test, dependency.
Fix | Fix an issue e.g. bug, typo, accident, misstatement.
Bump | Increase the version of something e.g. dependency.
Make | Change the build process, or tooling, or infra.
Start | Begin doing something; e.g. create a feature flag.
Stop | End doing something; e.g. remove a feature flag.
Refactor | A code change that MUST be just a refactoring.
Reformat | Refactor of formatting, e.g. omit whitespace.
Optimize | Refactor of performance, e.g. speed up code.
Document | Refactor of documentation, e.g. help files.

Subject lines must never contain (and / or start with) anything else. Especially not something that's unique to your system
