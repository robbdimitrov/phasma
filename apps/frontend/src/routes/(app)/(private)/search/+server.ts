import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { search, type SearchType } from '$lib/server/api/search';
import { apiClient } from '$lib/server/api/client';

const VALID_TYPES = new Set<string>(['users', 'posts', 'hashtags']);
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

	const page = await search(apiClient(event), {
		q,
		type: typeParam as SearchType,
		cursor,
		limit
	});
	return json(page);
};
