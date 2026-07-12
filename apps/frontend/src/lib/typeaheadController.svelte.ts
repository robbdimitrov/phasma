import { activeToken, type ActiveToken } from './utils/activeToken';

const DEBOUNCE_MS = 150;

export function createTypeaheadController() {
	let items = $state<unknown[]>([]);
	let token = $state<ActiveToken | null>(null);
	let debounceTimer: ReturnType<typeof setTimeout> | undefined;

	async function fetchSuggestions(forToken: ActiveToken) {
		const type = forToken.trigger === '@' ? 'users' : 'hashtags';
		try {
			const res = await fetch(
				`/suggest?type=${type}&q=${encodeURIComponent(forToken.query || ' ')}`
			);
			// A debounced request for an older token can resolve after a newer one
			// has superseded it; only apply results that still match.
			if (token !== forToken) return;
			items = res.ok ? ((await res.json()) as unknown[]) : [];
		} catch {
			if (token === forToken) items = [];
		}
	}

	function handleInput(value: string, caret: number) {
		const next = activeToken(value, caret);
		token = next;

		clearTimeout(debounceTimer);
		if (next && next.query.length > 0) {
			debounceTimer = setTimeout(() => fetchSuggestions(next), DEBOUNCE_MS);
		} else {
			items = [];
		}
	}

	function displayItem(item: unknown): string {
		if (token?.trigger === '@') {
			return (item as { username: string }).username;
		}
		return (item as { name: string }).name;
	}

	// Splices `selected` into `text` in place of the active token and restores
	// the caret on `el` after it. Returns null (leaving `text` untouched) when
	// there is no active token or `selected` is empty (Escape), so a stale or
	// dismissed suggestion can't corrupt the field or steal focus.
	function select(
		text: string,
		selected: string,
		el: HTMLInputElement | HTMLTextAreaElement | null
	): string | null {
		if (!token || !selected) {
			reset();
			return null;
		}
		const { trigger, start, end } = token;
		const inserted = trigger + selected + ' ';
		const next = text.slice(0, start) + inserted + text.slice(end);
		const caret = start + inserted.length;
		reset();

		requestAnimationFrame(() => {
			el?.setSelectionRange(caret, caret);
			el?.focus();
		});

		return next;
	}

	function reset() {
		clearTimeout(debounceTimer);
		items = [];
		token = null;
	}

	return {
		get items() {
			return items;
		},
		get token() {
			return token;
		},
		handleInput,
		displayItem,
		select,
		reset
	};
}
