import { describe, it, expect, vi } from 'vitest';
import { createPagination } from './createPagination.svelte';

// Source-change reset relies on `$effect`, which is a no-op in these SSR-mode
// unit tests; integration/manual testing covers that path.

type Page<T> = { items: T[]; nextCursor: string | null };

describe('createPagination', () => {
	it('exposes initial items and a pending cursor as not done', () => {
		const cleanup = $effect.root(() => {
			const p = createPagination<number>(
				() => ({ items: [1, 2], nextCursor: 'c1' }),
				async () => ({ items: [], nextCursor: null })
			);
			expect(p.items).toEqual([1, 2]);
			expect(p.done).toBe(false);
			expect(p.loading).toBe(false);
			expect(p.error).toBeNull();
		});
		cleanup();
	});

	it('appends the next page and advances the cursor', async () => {
		let p!: ReturnType<typeof createPagination<number>>;
		const cleanup = $effect.root(() => {
			p = createPagination<number>(
				() => ({ items: [1], nextCursor: 'c1' }),
				async (cursor) => {
					expect(cursor).toBe('c1');
					return { items: [2, 3], nextCursor: null };
				}
			);
		});
		await p.more();
		expect(p.items).toEqual([1, 2, 3]);
		expect(p.done).toBe(true);
		cleanup();
	});

	it('does not fetch when there is no cursor', async () => {
		const fetchPage = vi.fn<(c: string) => Promise<Page<number>>>();
		let p!: ReturnType<typeof createPagination<number>>;
		const cleanup = $effect.root(() => {
			p = createPagination<number>(() => ({ items: [1], nextCursor: null }), fetchPage);
		});
		await p.more();
		expect(fetchPage).not.toHaveBeenCalled();
		expect(p.done).toBe(true);
		cleanup();
	});

	it('captures the error message when a page fetch rejects', async () => {
		let p!: ReturnType<typeof createPagination<number>>;
		const cleanup = $effect.root(() => {
			p = createPagination<number>(
				() => ({ items: [1], nextCursor: 'c1' }),
				async () => {
					throw new Error('boom');
				}
			);
		});
		await p.more();
		expect(p.error).toBe('boom');
		expect(p.loading).toBe(false);
		expect(p.items).toEqual([1]);
		cleanup();
	});
});
