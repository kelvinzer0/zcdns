# ZCDNS Landing Page & Documentation

This repository hosts the codebase for the ZCDNS landing page and its integrated documentation portal. Built with modern web technologies, it provides a responsive and internationalized user experience.

## Core Technologies

*   **React**: A JavaScript library for building user interfaces.
*   **TypeScript**: A superset of JavaScript that adds static typing.
*   **Vite**: A fast build tool that provides a lightning-fast development experience.
*   **Tailwind CSS**: A utility-first CSS framework for rapidly building custom designs.
*   **MDX**: Allows writing JSX inside Markdown documents, used for dynamic documentation.
*   **i18n (Internationalization)**: Support for multiple languages (currently English and Indonesian).

## Project Structure Overview

The project is organized to separate concerns and facilitate maintainability:

```
.
├── content/
│   └── docs/               # Source MDX files for documentation, organized by language.
│       ├── en/
│       └── id/
├── public/
│   ├── docs/               # Generated JSON documentation files (from content/docs).
│   ├── mock-api/           # Mock API data for development.
│   └── ...                 # Other static assets (images, etc.).
├── scripts/
│   ├── build-docs-to-json.js   # Script to process MDX docs into JSON for the frontend.
│   └── generate-static-routes.js # Script for generating static routes based on docs.
├── src/
│   ├── assets/             # Static assets like images and SVGs used in the application.
│   ├── components/         # Reusable React components.
│   │   ├── ui/             # Generic, unstyled UI primitives (buttons, cards, etc.).
│   │   └── ...             # Application-specific components (header, footer, doc pages).
│   ├── hooks/              # Custom React hooks for shared logic.
│   ├── i18n/               # Internationalization setup and context.
│   ├── lib/                # Utility functions and helper modules.
│   ├── messages/           # JSON files containing translated strings for i18n.
│   └── ...                 # Main application entry points and core logic.
└── ...                     # Configuration files (vite.config.ts, tailwind.config.ts, tsconfig.json, package.json, etc.)
```

## Key Components & Features

*   **Documentation System**: Documentation is written in MDX (`content/docs/`) and processed by `scripts/build-docs-to-json.js` into JSON files (`public/docs/`) consumed by the React frontend. This allows for rich, interactive documentation pages.
*   **Internationalization (i18n)**: The application supports multiple languages. Translations are managed in `src/messages/` and integrated via `src/i18n/`. The `LanguageSwitcher` component (`src/components/LanguageSwitcher.tsx`) handles language selection.
*   **Modular UI Components**: The `src/components/` directory houses a wide array of reusable components, from generic UI elements (`src/components/ui/`) to complex application sections like `DocPageWrapper` and `Statistics`.
*   **Routing**: Static routes are likely generated and managed to serve both the landing page and the dynamic documentation pages.
*   **Theming**: Tailwind CSS is configured via `tailwind.config.ts` to provide a consistent design system.

## Contribution

Contributions are welcome. Please adhere to the existing code style, commit message guidelines, and testing practices. For major changes, please open an issue first to discuss what you would like to change.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
