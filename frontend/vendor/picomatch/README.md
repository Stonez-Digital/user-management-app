# Local picomatch security fork

This package is based on the MIT-licensed `picomatch@4.0.7` source.

The only behavioral change is a narrow repeated-extglob guard adapted from
[upstream PR #238](https://github.com/micromatch/picomatch/pull/238), which
addresses CVE-2026-112366 / GHSA-fc5q-39xc-vjpf by literalizing single-branch
repeated extglobs whose branch begins with an unparenthesized `*` or `?`
followed by more content.

The upstream PR is not merged and no official fixed npm release is available.
This fork is therefore an interim, locally maintained patch—not an upstream
release. The regression tests live at `frontend/tests/picomatch-redos.test.mjs`.

When the picomatch maintainers publish an official fixed release, remove this
vendor directory and the npm override, upgrade to that release, and retain the
regression tests.
