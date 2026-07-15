// Fire-and-forget: recording a recent search is a non-critical side effect of
// navigation and must never block or fail it.
export function recordRecentSearch(type: 'users' | 'hashtags' | 'posts', reference: string): void {
	fetch('/search/recent', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ type, reference })
	}).catch(() => {});
}
