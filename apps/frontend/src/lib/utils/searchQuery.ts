export const SEARCH_PREVIEW_LIMIT = 5;

export type SearchQueryPrefix = '@' | '#' | null;

export function searchQueryPrefix(rawQuery: string): SearchQueryPrefix {
	if (rawQuery.startsWith('@')) return '@';
	if (rawQuery.startsWith('#')) return '#';
	return null;
}

export function stripSearchQueryPrefix(rawQuery: string, prefix: SearchQueryPrefix): string {
	return prefix ? rawQuery.slice(1) : rawQuery;
}
