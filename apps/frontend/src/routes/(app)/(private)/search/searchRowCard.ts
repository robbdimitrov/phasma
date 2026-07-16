// Shared "row card" look for a search result: the live typeahead's rows
// (SearchResultRow.svelte, card mode) and each recent-search entry's <li>
// (RecentSearches.svelte) render the same card so the two dropdowns read as
// one visual language.
export const SEARCH_ROW_CARD_CLASS =
	'flex items-center gap-3 rounded-2xl border border-base-300 bg-base-100 p-3 transition-colors hover:bg-base-200';
