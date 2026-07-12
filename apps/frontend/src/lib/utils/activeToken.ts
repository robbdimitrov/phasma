export type ActiveToken = {
	trigger: '@' | '#';
	query: string;
	start: number;
	end: number;
};

// Superset scan charset (matches the username pattern); hashtags are
// narrower (backend extracts only `[A-Za-z0-9_]`) and filtered below.
const TOKEN_CHAR_RE = /[A-Za-z0-9._]/;
const HASHTAG_CHAR_RE = /^[A-Za-z0-9_]*$/;

/**
 * Given a text value and the current caret position, returns the active
 * @mention or #hashtag token the caret is within, or null if none.
 *
 * `start` is the index of the trigger character, `end` is the caret position.
 */
export function activeToken(value: string, caret: number): ActiveToken | null {
	// Scan left over token characters.
	let i = caret - 1;
	while (i >= 0 && TOKEN_CHAR_RE.test(value.charAt(i))) {
		i--;
	}

	// The character immediately before the run must be @ or #.
	const triggerChar = value.charAt(i);
	if (triggerChar !== '@' && triggerChar !== '#') return null;

	const triggerIndex = i;
	const trigger = triggerChar;

	// The trigger must be at the start of the string or preceded by whitespace.
	if (triggerIndex > 0 && !/\s/.test(value.charAt(triggerIndex - 1))) return null;

	const query = value.slice(triggerIndex + 1, caret);
	if (trigger === '#' && !HASHTAG_CHAR_RE.test(query)) return null;

	return {
		trigger,
		query,
		start: triggerIndex,
		end: caret
	};
}
