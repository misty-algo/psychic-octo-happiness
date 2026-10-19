# Web Instructions

This directory contains the Echo Admin frontend.

- Keep the frontend aligned with the backend admin API under `/api`.
- Do not restore upstream demo pages, generated services, request-record mocks, or generator-first workflows.
- Put API methods and DTO types in `src/services/admin.ts`.
- Keep pages thin: forms, tables, state, and DTO conversion only.
- Do not commit generated output such as `dist`, `.umi*`, or `.turbopack`.
- Use `npm run lint`, `npm run test`, and `npm run build` for verification.
