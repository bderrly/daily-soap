This is a web application for personal devotion and journaling using Go server, JavaScript and AJAX via htmx.
This project does not use on Node. Do not install NPM packages.
This project uses `mise` to manage project-specific tools. Run tools from mise.toml files with `mise exec`.
Lint the project after editing files. For linting conventions, see docs/LINTING.md.

## Go

- Add document comments on all exported Go functions and methods.
- Follow the Effective Go guidelines.
- Prioritize standard library packages rather than custom code.
- Use "_test" packages whenever possible when writing unit tests.
