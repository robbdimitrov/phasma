import { json, type RequestEvent } from '@sveltejs/kit';
import { apiClient, type ApiClient } from '$lib/server/api/client';

type CursorEndpointEvent = Pick<RequestEvent, 'fetch' | 'cookies' | 'url'>;

interface CursorEndpointOptions {
	/**
	 * Defaults to requiring a session. Use 'public' only when the initial page
	 * is already anonymous-readable and pagination must match it.
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
