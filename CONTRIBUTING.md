# Code of Conduct

We are committed to creating a welcoming and inclusive environment for all contributors.
As such, we have adopted the following code of conduct, which all contributors are expected to follow:

- Be respectful of others. Harassment or discrimination of any kind will not be tolerated.
- Be mindful of your language and behavior when communicating with others.
- Be open-minded and willing to learn from others.

# Contributing Guidelines

We welcome contributions to this repository. Here are some guidelines to follow when contributing:

- Make sure your code follows the [Effective Go](https://golang.org/doc/effective_go.html).
- Run the tests and make sure they pass before submitting your changes.
- Use clear and descriptive commit messages.
- Make sure you ran linters before submitting your code:
  - `goimports -w ./`
  - `gofmt -s -w ./`
  - `golangci-run run ./...`
- Create a pull request, follow the PR template to fill the description.
- The maintainers will review your pull request and may request changes or ask questions.
    - <details><summary>The `git log` of you PR is also a subject of review, and you could be asked to fix it by squashing with a meaningful message or simply amending commits in the way that you see it fit.</summary>

      Try to maintain a clean `git log` within your branch.
      You may separate your changes into several commits to make the PR reviews easier.

      If you push small commits addressing review comments to fix a small bug or typo, those should be squashed before merging.
      It helps to think of you commits as something that will be easy to revert later, or bisect to identify bugs.
      A commit for `some change` will be easier to handle than 3 commits about `some change` + `small fix` + `fix typo`.

      </details>
    - Try to rebase your PR against main on a regular basis.
    - Try to avoid merging in main your branch instead of rebasing.
- Merge your PR after getting at least one approval (two are preferable on larger PRs).
