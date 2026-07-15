import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { recordRecentSearch } from '$lib/server/api/search';
import { apiClient } from '$lib/server/api/client';

const VALID_TYPES = new Set(['users', 'hashtags', 'posts']);
const MAX_REFERENCE_LENGTH = 50;

export const POST: RequestHandler = async (event) => {
	if (!event.cookies.get('session')) return new Response(null, { status: 401 });

	let body: unknown;
	try {
		body = await event.request.json();
	} catch {
		throw error(400, 'Malformed JSON request body.');
	}
	const { type, reference: rawReference } = (body ?? {}) as {
		type?: unknown;
		reference?: unknown;
	};

	if (typeof type !== 'string' || !VALID_TYPES.has(type)) {
		throw error(400, 'Invalid search type.');
	}
	// Trimmed here too (in addition to the backend) so this BFF boundary
	// rejects whitespace-only input the same way a direct backend caller
	// would, rather than forwarding it and relying solely on the backend.
	const reference = typeof rawReference === 'string' ? rawReference.trim() : '';
	if (reference.length < 1 || reference.length > MAX_REFERENCE_LENGTH) {
		throw error(400, 'Reference must be 1–50 characters.');
	}

	await recordRecentSearch(apiClient(event), type, reference);
	return new Response(null, { status: 204 });
};
