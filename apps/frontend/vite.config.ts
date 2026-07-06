import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	// Vitest needs Svelte's client build for jsdom mount/unmount helpers.
	resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined,
	test: {
		include: ['src/**/*.{test,spec}.{js,ts}'],
		environment: 'jsdom',
		globals: true
	}
});
