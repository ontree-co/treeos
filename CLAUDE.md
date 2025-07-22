1. The Claude.md files are not to be touched by the agents. Only when explicitly requested.
2. This repository has a pre-commit hook installed that prevents commits with linter or unit test errors. It is forbidden to circumvent this in any way. Instead the tests and linting are to be fixed.
3. This app does not need backwards compatibility to earlier versions. The goal is to have a simple solution rather.
4. Restrain from defensive fixes for issues. In this code base most of the time there is a right place for the fix. Take your time to locate it and suggest to fix it there.
5. Add a unit test first if applicable to really find out if your fix fixes the problem.
6. This project supports macOS on Apple Silicon and Linux on amd64 and arm64.
