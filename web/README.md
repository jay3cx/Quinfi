# FundMind Web

FundMind Web 前端（React + TypeScript + Vite）。

## Stack

- React 19
- TypeScript 5
- Vite 7
- Zustand
- TailwindCSS 4

## Development

```bash
cd web
npm ci
npm run dev
```

默认开发地址：`http://localhost:5173`

开发环境通过 `vite.config.ts` 代理到后端 `http://localhost:8080`。

## Build

```bash
npm run build
npm run preview
```

## Lint

```bash
npm run lint
```

## Project Layout

```text
src/
├── components/
├── pages/
├── hooks/
├── stores/
├── lib/
├── types/
├── App.tsx
└── main.tsx
```
