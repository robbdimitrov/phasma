import type { RequestEvent } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';

/** A backend-bound fetch: a backend-relative path plus the standard `RequestInit`. */
export type ApiClient = (path: string, init?: RequestInit) => Promise<Response>;

const backendBase = (): string => env.BACKEND_URL ?? 'http://localhost:8080';

/**
 * Per-request BFF client that forwards the session cookie server-side; the
 * browser remains limited to this origin by CSP.
 */
export function apiClient(event: Pick<RequestEvent, 'fetch' | 'cookies'>): ApiClient {
	return (path, init) => {
		const headers = new Headers(init?.headers);
		// Read per call to honour a session set earlier in the same request.
		const session = event.cookies.get('session');
		if (session) headers.set('cookie', `session=${session}`);
		// Deliberately the global fetch, not event.fetch: event.fetch always adds
		// an Origin header, which trips the backend's cross-site POST guard on
		// this trusted, server-to-server call.
		return fetch(new URL(path, backendBase()), { ...init, headers });
	};
}
