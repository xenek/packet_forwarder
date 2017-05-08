.PHONY: hooks

GIT_COMMIT = `git rev-parse HEAD 2>/dev/null`
GIT_BRANCH = `git rev-parse --abbrev-ref HEAD 2>/dev/null`
GIT_TAG = $(shell git describe --abbrev=0 --tags)
BUILD_DATE = `date -u +%Y-%m-%dT%H:%M%SZ`

# Get all files that are currently staged, except for deleted files
STAGED_FILES = git diff --staged --name-only --diff-filter=d


# Install git hooks
hooks:
	@$(log) installing hooks
	@touch .git/hooks/pre-commit
	@chmod u+x .git/hooks/pre-commit
	@echo "make quality-staged" >> .git/hooks/pre-commit
