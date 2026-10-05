# Nuxt Minimal Starter

Look at the [Nuxt documentation](https://nuxt.com/docs/getting-started/introduction) to learn more.

## Setup

Make sure to install dependencies:

```bash
# npm
npm install

# pnpm
pnpm install

# yarn
yarn install

# bun
bun install
```

## Development Server

Start the development server on `http://localhost:3000`:

```bash
# npm
npm run dev

# pnpm
pnpm dev

# yarn
yarn dev

# bun
bun run dev
```

## Production

Build the application for production:

```bash
# npm
npm run build

# pnpm
pnpm build

# yarn
yarn build

# bun
bun run build
```

Locally preview production build:

```bash
# npm
npm run preview

# pnpm
pnpm preview

# yarn
yarn preview

# bun
bun run preview
```

## OAuth2 Proxy configuration

The Compose `oauth2-proxy1` service reads its Authentik client and cookie secrets from a private env file. Keep that file outside the repository and provide it explicitly when starting the proxy:

```dotenv
BOOPS_OIDC_CLIENT_SECRET=<Authentik client secret>
BOOPS_COOKIE_SECRET=<oauth2-proxy cookie secret>
```

```bash
docker compose --env-file /path/to/boops-proxy.env \
  up -d --no-deps --pull never --no-build --force-recreate oauth2-proxy1
```

Do not commit the env file or inline either secret in Compose.

Check out the [deployment documentation](https://nuxt.com/docs/getting-started/deployment) for more information.
