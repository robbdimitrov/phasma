import type { ApiClient } from './client';
import { unwrap } from './http';

export interface UserSuggestion {
	username: string;
	name: string;
	avatar: string | null;
}

export interface HashtagSuggestion {
	name: string;
	postCount: number;
}

export async function searchUsers(fetch: ApiClient, q: string): Promise<UserSuggestion[]> {
	const res = await fetch(`/users/search?q=${encodeURIComponent(q)}`);
	return (await unwrap<UserSuggestion[]>(res)) ?? [];
}

export async function searchHashtags(fetch: ApiClient, q: string): Promise<HashtagSuggestion[]> {
	const res = await fetch(`/hashtags/search?q=${encodeURIComponent(q)}`);
	return (await unwrap<HashtagSuggestion[]>(res)) ?? [];
}

export type SearchType = 'users' | 'posts' | 'hashtags' | 'all';

export interface SearchUserItem {
	username: string;
	name: string;
	avatar: string | null;
}

export interface SearchPostItem {
	id: string;
	username: string;
	description: string;
	filename: string;
}

export interface SearchHashtagItem {
	name: string;
	postCount: number;
}

export type SearchItem = SearchUserItem | SearchPostItem | SearchHashtagItem;

// A blended (type=all) result: one entity tagged with its type, matching the
// backend's BlendedItem wire shape.
export type SearchAllItem =
	| { type: 'users'; item: SearchUserItem }
	| { type: 'posts'; item: SearchPostItem }
	| { type: 'hashtags'; item: SearchHashtagItem };

export interface SearchPage<T = SearchItem> {
	items: T[];
	nextCursor: string | null;
}

export async function search<T = SearchItem>(
	fetch: ApiClient,
	params: { q: string; type: SearchType; cursor?: string; limit?: number }
): Promise<SearchPage<T>> {
	const qs = new URLSearchParams({ q: params.q, type: params.type });
	if (params.cursor) qs.set('cursor', params.cursor);
	if (params.limit) qs.set('limit', String(params.limit));
	const res = await fetch(`/search?${qs.toString()}`);
	const page = await unwrap<SearchPage<T>>(res);
	return { items: page?.items ?? [], nextCursor: page?.nextCursor ?? null };
}

// Mirrors SearchAllItem's {type, item} discriminated union, but the `posts`
// variant here is the raw submitted query text, not a real post.
export type RecentSearchItem =
	| { id: string; type: 'users'; item: SearchUserItem }
	| { id: string; type: 'hashtags'; item: SearchHashtagItem }
	| { id: string; type: 'posts'; item: string };

export async function listRecentSearches(fetch: ApiClient): Promise<RecentSearchItem[]> {
	const res = await fetch('/search/recent');
	return (await unwrap<RecentSearchItem[]>(res)) ?? [];
}

export async function recordRecentSearch(
	fetch: ApiClient,
	type: string,
	reference: string
): Promise<null> {
	const res = await fetch('/search/recent', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ type, reference })
	});
	return unwrap<null>(res);
}

export async function deleteRecentSearch(fetch: ApiClient, id: string): Promise<null> {
	const res = await fetch(`/search/recent/${encodeURIComponent(id)}`, { method: 'DELETE' });
	return unwrap<null>(res);
}

export async function clearRecentSearches(fetch: ApiClient): Promise<null> {
	const res = await fetch('/search/recent', { method: 'DELETE' });
	return unwrap<null>(res);
}
