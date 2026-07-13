import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { search, type SearchAllItem, type SearchItem, type SearchType } from '$lib/server/api/search';
import { apiClient } from '$lib/server/api/client';

const VALID_TYPES = new Set<string>(['users', 'posts', 'hashtags', 'all']);
const MAX_Q_LENGTH = 50;

export const GET: RequestHandler = async (event) => {
	if (!event.cookies.get('session')) return new Response(null, { status: 401 });
	const q = event.url.searchParams.get('q') ?? '';
	const typeParam = event.url.searchParams.get('type') ?? 'posts';
	const cursor = event.url.searchParams.get('cursor') ?? undefined;
	const limitParam = event.url.searchParams.get('limit');
	const limit = limitParam ? Number(limitParam) : undefined;

	if (!q || q.length > MAX_Q_LENGTH || !VALID_TYPES.has(typeParam)) {
		return json({ items: [], nextCursor: null });
	}

	const type = typeParam as SearchType;
	if (type === 'all') {
		return json(await search<SearchAllItem>(apiClient(event), { q, type, cursor, limit }));
	}

	// A "Load more" continuation for a single-type page (the dropdown's posts
	// preview, or a @/# prefix-narrowed search) still wraps items so the
	// browser only ever renders one shape of blended item.
	const page = await search<SearchItem>(apiClient(event), { q, type, cursor, limit });
	return json({
		items: page.items.map((item) => ({ type, item }) as SearchAllItem),
		nextCursor: page.nextCursor
	});
};
