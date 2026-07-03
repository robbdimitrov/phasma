import { json, type RequestEvent } from '@sveltejs/kit';
import { apiClient, type ApiClient } from '$lib/server/api/client';

type CursorEndpointEvent = Pick<RequestEvent, 'fetch' | 'cookies' | 'url'>;

interface CursorEndpointOptions {
	/**
	 * Defaults to requiring a session (deny by default). Set to 'public' only
	 * for endpoints whose backend route is intentionally public (e.g. post
	 * comments, a profile's posts/likes/followers/following) — the initial
	 * page for these is already served to anonymous visitors via `load`, so
	 * pagination must not require a session either.
	 */
	auth?: 'session' | 'public';
}

export async function cursorEndpoint<T>(
	event: CursorEndpointEvent,
	loadPage: (client: ApiClient, cursor: string | undefined) => Promise<T>,
	{ auth = 'session' }: CursorEndpointOptions = {}
) {
	if (auth === 'session' && !event.cookies.get('session')) {
		return new Response(null, { status: 401 });
	}
	const cursor = event.url.searchParams.get('cursor') ?? undefined;
	return json(await loadPage(apiClient(event), cursor));
}
