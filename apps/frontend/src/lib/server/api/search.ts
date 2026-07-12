import type { ApiClient } from './client';
import { unwrap } from './http';

export interface UserSuggestion {
	username: string;
	name: string;
	avatar: string | null;
}

export interface HashtagSuggestion {
	name: string;
}

export async function searchUsers(fetch: ApiClient, q: string): Promise<UserSuggestion[]> {
	const res = await fetch(`/users/search?q=${encodeURIComponent(q)}`);
	return (await unwrap<UserSuggestion[]>(res)) ?? [];
}

export async function searchHashtags(fetch: ApiClient, q: string): Promise<HashtagSuggestion[]> {
	const res = await fetch(`/hashtags/search?q=${encodeURIComponent(q)}`);
	return (await unwrap<HashtagSuggestion[]>(res)) ?? [];
}

export type SearchType = 'users' | 'posts' | 'hashtags';

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

export interface SearchPage<T extends SearchItem = SearchItem> {
	items: T[];
	nextCursor: string | null;
}

export async function search<T extends SearchItem = SearchItem>(
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
