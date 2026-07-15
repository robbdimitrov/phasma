import type { Handle, HandleServerError } from '@sveltejs/kit';
import { parseTheme } from '$lib/theme';

export const handle: Handle = async ({ event, resolve }) => {
	const theme = parseTheme(event.cookies.get('theme'));
	const res = await resolve(event, {
		transformPageChunk: ({ html }) => html.replace('data-theme="system"', `data-theme="${theme}"`)
	});
	res.headers.set('X-Content-Type-Options', 'nosniff');
	res.headers.set('X-Frame-Options', 'DENY');
	res.headers.set('Referrer-Policy', 'strict-origin-when-cross-origin');
	res.headers.set('Cross-Origin-Opener-Policy', 'same-origin');
	res.headers.set(
		'Permissions-Policy',
		'camera=(), microphone=(), geolocation=(), payment=(), usb=(), bluetooth=()'
	);
	// No Strict-Transport-Security: this deployment has no TLS termination,
	// and sending it over plain HTTP would be a false guarantee to clients.
	return res;
};

export const handleError: HandleServerError = ({ error, event }) => {
	const message = error instanceof Error ? error.message : 'Internal error';
	console.error(`[error] ${event.request.method} ${event.url.pathname}: ${message}`);
	return { message: 'Internal error' };
};
