## Pre-review checklist

<!--
    Make sure you took care of the issues on the list.
    Put 'x' into those boxes which apply.
    You can also create the PR now and click on all relevant checkboxes.
    See the review rules in CONTRIBUTING.md for more details.

    Do not remove any items from the list.
    Do not modify existing text of the list. If needed, add more text to existing points.
-->

- [ ] The PR title follows `type(scope): subject` from CONTRIBUTING.md. The PR is squash-merged, so the title becomes the commit subject on `main`.
- [ ] PR description sums up the changes and reasons why they should be introduced. It becomes the commit body on `main`.
- [ ] The code compiles, passes static checks and passes tests.
- [ ] I (human) have personally reviewed every change in this PR. I understand what it does and can explain why each change was made.
- [ ] I take full ownership of this code. "The AI wrote it" is not an acceptable answer to questions about it.
- [ ] I added relevant tests for new features and bug fixes.
  - [ ] N/A (Nothing to add tests for)
- [ ] A resource made by the last release plans "No changes" with this build.
  - [ ] N/A (No schema or resource behaviour change)
- [ ] Each new or changed field is ForceNew or really updated in place, and its description says if a change replaces the resource.
  - [ ] N/A (No field was added or changed)
- [ ] A renamed field or a changed type has a `SchemaVersion` increase and a state upgrader.
  - [ ] N/A (No field was renamed and no type was changed)
- [ ] A breaking change has `!` in the PR title (`feat!:`) and goes only into a major version.
  - [ ] N/A (No breaking change)
- [ ] I have adjusted `examples/` or `templates/` and ran `make generate`.
  - [ ] N/A (The PR doesn't need any docs changes)
- [ ] I added a changelog entry in `.changelog/<PR number>.txt`.
  - [ ] N/A (No user-visible change. The PR has the `no-changelog` label)
- [ ] I added appropriate `Fixes:` annotations to PR description.
  - [ ] N/A (The PR doesn't need an issue)
