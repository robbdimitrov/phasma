import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	// Resolve the "browser" package export condition under Vitest so Svelte
	// components compile in client mode, matching the jsdom test environment
	// (mount/unmount from 'svelte' require the client build, not the SSR build
	// production requests otherwise need).
	resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined,
	test: {
		include: ['src/**/*.{test,spec}.{js,ts}'],
		environment: 'jsdom',
		globals: true
	}
});
