# OnTree Documentation

This is the official documentation for OnTree, built with [Docusaurus](https://docusaurus.io/).

## Installation

```bash
npm install
# or
yarn
```

## Local Development

```bash
npm start
# or
yarn start
```

This command starts a local development server and opens up a browser window at http://localhost:3000. Most changes are reflected live without having to restart the server.

## Build

```bash
npm run build
# or
yarn build
```

This command generates static content into the `build` directory and can be served using any static contents hosting service.

## Deployment

Using SSH:

```bash
USE_SSH=true yarn deploy
```

Not using SSH:

```bash
GIT_USER=<Your GitHub username> yarn deploy
```

If you are using GitHub pages for hosting, this command is a convenient way to build the website and push to the `gh-pages` branch.

## Structure

- `/docs` - Main documentation content
  - `/getting-started` - Installation and setup guides
  - `/features` - Feature documentation
  - `/reference` - Configuration and API reference
- `/blog` - Changelog and announcements
- `/src` - Custom React components and pages
- `/static` - Static assets like images

## Contributing

When contributing to the documentation:

1. Write in clear, simple English
2. Include code examples where appropriate
3. Test all commands and code snippets
4. Add screenshots for UI-related features
5. Update the sidebar navigation if adding new pages

## Writing Guidelines

- Use present tense ("OnTree provides" not "OnTree will provide")
- Be concise but complete
- Include practical examples
- Link to related documentation
- Keep screenshots up to date

## License

The documentation is licensed under the same license as the OnTree project.