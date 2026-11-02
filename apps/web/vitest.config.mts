import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

// tsconfig keeps JSX for Next, so Vitest needs its own JSX transform.
export default defineConfig({ plugins: [react()] });
