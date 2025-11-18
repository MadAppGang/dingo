# Dingo Landing Page

Official landing page for [Dingo](https://dingolang.com) - a meta-language for Go with Result types, pattern matching, and error propagation.

Built with [Astro](https://astro.build) for optimal performance and SEO.

## Features

- **Server-first rendering**: Fast page loads with minimal JavaScript
- **Islands Architecture**: Interactive components only where needed
- **Firebase Authentication**: GitHub and Google OAuth sign-in
- **GitHub Pages deployment**: Automated CI/CD with GitHub Actions
- **Content Collections**: Type-safe golden test examples from Dingo transpiler
- **Responsive design**: Mobile-friendly UI

## Tech Stack

- **Framework**: Astro 5.x
- **UI Library**: React (for interactive islands)
- **Styling**: Tailwind CSS v4
- **Authentication**: Firebase Auth
- **State Management**: nanostores
- **Hosting**: GitHub Pages
- **Package Manager**: pnpm

## Project Structure

```
/
├── .github/
│   └── workflows/
│       └── deploy.yml          # GitHub Pages deployment
├── public/
│   ├── favicon.svg
│   └── og-image.png            # Open Graph image
├── src/
│   ├── assets/                 # Optimized images
│   ├── components/
│   │   └── react/              # React islands
│   │       ├── App.tsx         # Main app component
│   │       ├── AuthStateListener.tsx
│   │       ├── SignInButton.tsx
│   │       ├── UserMenu.tsx
│   │       └── AuthError.tsx
│   ├── content/
│   │   └── golden-examples/    # Dingo code examples
│   ├── layouts/
│   │   └── BaseLayout.astro    # Base HTML layout
│   ├── lib/
│   │   └── firebase.ts         # Firebase config
│   ├── pages/
│   │   └── index.astro         # Homepage
│   ├── stores/
│   │   └── auth.ts             # Auth state (nanostores)
│   └── styles/
├── ai-docs/                    # AI agent knowledge base
├── .env.example                # Environment variable template
├── FIREBASE_SETUP.md           # Firebase setup guide
└── astro.config.mjs            # Astro configuration
```

## Getting Started

### Prerequisites

- Node.js 20+
- pnpm 10+
- Firebase project (see [FIREBASE_SETUP.md](./FIREBASE_SETUP.md))

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/dingo.git
   cd dingo/langingpage
   ```

2. Install dependencies:
   ```bash
   pnpm install
   ```

3. Set up Firebase Authentication:
   - Follow the complete guide in [FIREBASE_SETUP.md](./FIREBASE_SETUP.md)
   - Create `.env` file with Firebase config:
     ```bash
     cp .env.example .env
     # Edit .env with your Firebase credentials
     ```

4. Start development server:
   ```bash
   pnpm dev
   ```

5. Open http://localhost:4321

## Commands

All commands run from the project root:

| Command | Action |
|---------|--------|
| `pnpm install` | Install dependencies |
| `pnpm dev` | Start dev server at `localhost:4321` |
| `pnpm build` | Build production site to `./dist/` |
| `pnpm preview` | Preview production build locally |
| `pnpm astro ...` | Run Astro CLI commands |
| `pnpm astro check` | Check TypeScript and Astro syntax |

## Firebase Authentication

This project uses Firebase Authentication with:
- **Google OAuth** - Sign in with Google account
- **GitHub OAuth** - Sign in with GitHub account

### Local Development

1. Complete [FIREBASE_SETUP.md](./FIREBASE_SETUP.md) setup
2. Copy Firebase config to `.env`
3. Test both OAuth providers locally

### Production Deployment

1. Add Firebase config as GitHub repository secrets
2. Push to `main` branch
3. GitHub Actions automatically builds and deploys to GitHub Pages

See [FIREBASE_SETUP.md](./FIREBASE_SETUP.md) for detailed instructions.

## Deployment

### GitHub Pages (Recommended)

1. Enable GitHub Pages:
   - Go to **Settings** → **Pages**
   - Source: **GitHub Actions**

2. Configure secrets (see [FIREBASE_SETUP.md](./FIREBASE_SETUP.md#step-9-configure-github-secrets-for-deployment))

3. Push to main:
   ```bash
   git push origin main
   ```

GitHub Actions workflow (`.github/workflows/deploy.yml`) will automatically:
- Install dependencies
- Build Astro site
- Deploy to GitHub Pages

### Custom Domain

1. Add `CNAME` file to `public/`:
   ```
   dingolang.com
   ```

2. Configure DNS:
   - Add A records pointing to GitHub Pages IPs
   - Or CNAME record pointing to `username.github.io`

3. Update Firebase authorized domains with custom domain

## Architecture

### Astro Islands Pattern

Following Astro best practices (see `ai-docs/`):

- **Static content**: Server-rendered HTML (0 KB JavaScript)
- **Interactive islands**: React components with `client:*` directives
  - `client:load` - AuthStateListener (critical, loads immediately)
  - `client:idle` - SignInButton, UserMenu, AuthError (loads after page interactive)

### State Management

- **nanostores**: Lightweight (~300 bytes), framework-agnostic state
- **Auth state**: Shared across islands via `src/stores/auth.ts`
- **Firebase SDK**: Handles auth persistence in localStorage

### Performance

- **Bundle size**: ~61 KB JavaScript (Firebase + auth islands)
- **LCP target**: < 2.5s
- **Static HTML**: All non-auth content served as static HTML

## Content Management

Golden test examples from `../tests/golden/` are loaded as Astro Content Collections:

```typescript
const allExamples = await getCollection('golden-examples');
```

Examples showcase Dingo features:
- Result<T,E> types
- Error propagation with `?`
- Pattern matching
- Sum types (enums)

## AI Development

This project includes AI agent knowledge base in `ai-docs/`:
- Astro best practices
- Islands Architecture patterns
- Component development guidelines

See [CLAUDE.md](./CLAUDE.md) for AI agent instructions.

## Resources

- **Dingo Transpiler**: [github.com/dingo-lang/dingo](https://github.com/dingo-lang/dingo)
- **Astro Documentation**: [docs.astro.build](https://docs.astro.build)
- **Firebase Auth Docs**: [firebase.google.com/docs/auth](https://firebase.google.com/docs/auth)
- **GitHub Pages**: [docs.github.com/en/pages](https://docs.github.com/en/pages)

## License

MIT
