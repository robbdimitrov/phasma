import { error } from '@sveltejs/kit';
import type { CursorPage } from '$lib/types';
import type { ApiClient } from './client';

export function statusMessage(status: number): string {
	switch (status) {
		case 400:
			return 'The request could not be completed.';
		case 401:
			return 'Please sign in to continue.';
		case 403:
			return 'You do not have permission to do that.';
		case 404:
			return 'Not found.';
		case 409:
			return 'That change conflicts with the current state.';
		case 429:
			return 'Too many requests. Please try again later.';
		case 503:
			return 'The service is temporarily unavailable. Please try again later.';
		default:
			return status >= 500
				? 'Something went wrong. Please try again.'
				: 'The request could not be completed.';
	}
}

export async function unwrap<T>(res: Response): Promise<T | null> {
	if (res.status === 204) return null;
	if (!res.ok) {
		await res.body?.cancel().catch(() => {});
		throw error(res.status, statusMessage(res.status));
	}
	const text = await res.text();
	return text ? (JSON.parse(text) as T) : null;
}

export async function getCursorPage<Dto, T>(
	fetch: ApiClient,
	url: string,
	cursor: string | null | undefined,
	map: (dto: Dto) => T
): Promise<CursorPage<T>> {
	const params = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const res = await fetch(`${url}${params}`);
	const page = await unwrap<{ items: Dto[]; nextCursor: string | null }>(res);
	return { items: (page?.items ?? []).map(map), nextCursor: page?.nextCursor ?? null };
}
