# Multibranch Pipeline Test

This file verifies Jenkins Multibranch Pipeline behaviour.

Expected result:

- Jenkins discovers the feature branch.
- Go tests and compilation run successfully.
- Container image publishing is skipped.
- Kubernetes deployment is skipped.
- The production version remains unchanged.
